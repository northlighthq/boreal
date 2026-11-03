package parser

import "unicode/utf8"

const (
	maxParams        = 32
	maxIntermediates = 4
	maxOSCLen        = 64 * 1024
	maxParamValue    = 65535
)

type parserState uint8

const (
	stateGround parserState = iota
	stateEscape
	stateEscapeIntermediate
	stateCSIEntry
	stateCSIParam
	stateCSIIntermediate
	stateCSIIgnore
	stateOSC
	stateStringIgnore
)

type Handler interface {
	Print(r rune)
	Execute(b byte)
	CSI(p *Params, intermediates []byte, final byte)
	ESC(intermediates []byte, final byte)
	OSC(data []byte)
}

type ASCIIPrinter interface {
	PrintASCII(s []byte)
}

type Params struct {
	vals [maxParams]int
	sub  [maxParams]bool
	n    int
}

func (p *Params) Len() int {
	return p.n
}

func (p *Params) Get(i, def int) int {
	if i >= p.n || p.vals[i] == 0 {
		return def
	}

	return p.vals[i]
}

func (p *Params) Raw(i int) int {
	if i >= p.n {
		return 0
	}

	return p.vals[i]
}

func (p *Params) IsSub(i int) bool {
	return i < p.n && p.sub[i]
}

func (p *Params) reset() {
	p.n = 0
}

type Parser struct {
	h     Handler
	ascii ASCIIPrinter
	state parserState

	params   Params
	hasDigit bool
	nextSub  bool
	inter    [maxIntermediates]byte
	interLen int
	osc      []byte
	utf8Buf  [utf8.UTFMax]byte
	utf8Len  int
	utf8Need int
}

func New(h Handler) *Parser {
	ps := &Parser{h: h}
	ps.ascii, _ = h.(ASCIIPrinter)
	return ps
}

func (ps *Parser) Feed(p []byte) {
	for i := 0; i < len(p); i++ {
		b := p[i]

		if ps.state != stateGround || ps.utf8Need != 0 || b < 0x20 || b >= 0x7f {
			ps.step(b)
			continue
		}

		if ps.ascii == nil {
			ps.h.Print(rune(b))
			continue
		}

		end := i + 1

		for end < len(p) && p[end] >= 0x20 && p[end] < 0x7f {
			end++
		}

		ps.ascii.PrintASCII(p[i:end])
		i = end - 1
	}
}

func (ps *Parser) step(b byte) {
	switch b {
	case 0x18, 0x1a:
		ps.abortUTF8()
		ps.h.Execute(b)
		ps.state = stateGround
		return
	case 0x1b:
		ps.abortUTF8()

		if ps.state == stateOSC {
			ps.h.OSC(ps.osc)
		}

		ps.clear()
		ps.state = stateEscape
		return
	}

	switch ps.state {
	case stateGround:
		ps.ground(b)
	case stateEscape:
		ps.escape(b)
	case stateEscapeIntermediate:
		ps.escapeIntermediate(b)
	case stateCSIEntry, stateCSIParam:
		ps.csiParam(b)
	case stateCSIIntermediate:
		ps.csiIntermediate(b)
	case stateCSIIgnore:
		ps.csiIgnore(b)
	case stateOSC:
		ps.oscString(b)
	case stateStringIgnore:
	}
}

func (ps *Parser) ground(b byte) {
	if b < 0x20 {
		ps.abortUTF8()
		ps.h.Execute(b)
		return
	}

	if b == 0x7f {
		return
	}

	if b < 0x80 {
		ps.abortUTF8()
		ps.h.Print(rune(b))
		return
	}

	ps.feedUTF8(b)
}

func (ps *Parser) feedUTF8(b byte) {
	if ps.utf8Need == 0 {
		switch {
		case b&0xe0 == 0xc0:
			ps.utf8Need = 2
		case b&0xf0 == 0xe0:
			ps.utf8Need = 3
		case b&0xf8 == 0xf0:
			ps.utf8Need = 4
		default:
			ps.h.Print(utf8.RuneError)
			return
		}

		ps.utf8Buf[0] = b
		ps.utf8Len = 1
		return
	}

	if b&0xc0 != 0x80 {
		ps.abortUTF8()
		ps.ground(b)
		return
	}

	ps.utf8Buf[ps.utf8Len] = b
	ps.utf8Len++

	if ps.utf8Len < ps.utf8Need {
		return
	}

	r, _ := utf8.DecodeRune(ps.utf8Buf[:ps.utf8Len])
	ps.utf8Len, ps.utf8Need = 0, 0
	ps.h.Print(r)
}

func (ps *Parser) abortUTF8() {
	if ps.utf8Need == 0 {
		return
	}

	ps.utf8Len, ps.utf8Need = 0, 0
	ps.h.Print(utf8.RuneError)
}

func (ps *Parser) escape(b byte) {
	switch {
	case b < 0x20:
		ps.h.Execute(b)
	case b <= 0x2f:
		ps.collect(b)
		ps.state = stateEscapeIntermediate
	case b == '[':
		ps.state = stateCSIEntry
	case b == ']':
		ps.osc = ps.osc[:0]
		ps.state = stateOSC
	case b == 'P', b == 'X', b == '^', b == '_':
		ps.state = stateStringIgnore
	case b < 0x7f:
		ps.h.ESC(ps.inter[:ps.interLen], b)
		ps.state = stateGround
	}
}

func (ps *Parser) escapeIntermediate(b byte) {
	switch {
	case b < 0x20:
		ps.h.Execute(b)
	case b <= 0x2f:
		ps.collect(b)
	case b < 0x7f:
		ps.h.ESC(ps.inter[:ps.interLen], b)
		ps.state = stateGround
	}
}

func (ps *Parser) csiParam(b byte) {
	switch {
	case b < 0x20:
		ps.h.Execute(b)
	case b >= '0' && b <= '9':
		ps.digit(b)
		ps.state = stateCSIParam
	case b == ';' || b == ':':
		ps.separator(b == ':')
		ps.state = stateCSIParam
	case b >= 0x3c && b <= 0x3f:
		if ps.state != stateCSIEntry {
			ps.state = stateCSIIgnore
			return
		}

		ps.collect(b)
		ps.state = stateCSIParam
	case b <= 0x2f:
		ps.collect(b)
		ps.state = stateCSIIntermediate
	case b >= 0x40 && b < 0x7f:
		ps.dispatchCSI(b)
	}
}

func (ps *Parser) csiIntermediate(b byte) {
	switch {
	case b < 0x20:
		ps.h.Execute(b)
	case b <= 0x2f:
		ps.collect(b)
	case b <= 0x3f:
		ps.state = stateCSIIgnore
	case b < 0x7f:
		ps.dispatchCSI(b)
	}
}

func (ps *Parser) csiIgnore(b byte) {
	switch {
	case b < 0x20:
		ps.h.Execute(b)
	case b >= 0x40 && b < 0x7f:
		ps.state = stateGround
	}
}

func (ps *Parser) oscString(b byte) {
	if b == 0x07 {
		ps.h.OSC(ps.osc)
		ps.state = stateGround
		return
	}

	if b < 0x20 || len(ps.osc) >= maxOSCLen {
		return
	}

	ps.osc = append(ps.osc, b)
}

func (ps *Parser) digit(b byte) {
	p := &ps.params

	if !ps.hasDigit {
		if p.n >= maxParams {
			return
		}

		p.vals[p.n] = 0
		p.sub[p.n] = ps.nextSub
		p.n++
		ps.hasDigit = true
	}

	v := &p.vals[p.n-1]
	*v = min(*v*10+int(b-'0'), maxParamValue)
}

func (ps *Parser) separator(sub bool) {
	p := &ps.params

	if !ps.hasDigit && p.n < maxParams {
		p.vals[p.n] = 0
		p.sub[p.n] = ps.nextSub
		p.n++
	}

	ps.hasDigit = false
	ps.nextSub = sub
}

func (ps *Parser) dispatchCSI(final byte) {
	ps.h.CSI(&ps.params, ps.inter[:ps.interLen], final)
	ps.state = stateGround
}

func (ps *Parser) collect(b byte) {
	if ps.interLen < maxIntermediates {
		ps.inter[ps.interLen] = b
		ps.interLen++
	}
}

func (ps *Parser) clear() {
	ps.params.reset()
	ps.hasDigit = false
	ps.nextSub = false
	ps.interLen = 0
}
