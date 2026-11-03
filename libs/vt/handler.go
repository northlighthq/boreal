package vt

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
)

const (
	oscTitleIcon = 0
	oscTitle     = 2
	oscCwd       = 7
	oscLink      = 8
	oscClipboard = 52
	maxTitleLen  = 256
)

type charset uint8

const (
	charsetASCII charset = iota
	charsetDECGraphics
)

var decGraphics = [...]rune{
	'◆', '▒', '␉', '␌', '␍', '␊', '°', '±', '␤', '␋', '┘', '┐', '┌', '└', '┼', '⎺',
	'⎻', '─', '⎼', '⎽', '├', '┤', '┴', '┬', '│', '≤', '≥', 'π', '≠', '£', '·',
}

func (t *Terminal) runeWidth(r rune) int {
	if r < 0x300 {
		return 1
	}

	if w, ok := t.widths[r]; ok {
		return int(w)
	}

	if t.widths == nil {
		t.widths = map[rune]uint8{}
	}

	w := uniseg.StringWidth(string(r))
	t.widths[r] = uint8(w)
	return w
}

func (t *Terminal) Print(r rune) {
	if t.cur.charsets[t.cur.gl] == charsetDECGraphics && r >= 0x60 && r <= 0x7e {
		r = decGraphics[r-0x60]
	}

	w := min(t.runeWidth(r), t.cols)

	if w == 0 {
		return
	}

	if t.cur.pendingWrap && t.modes.autowrap {
		t.screen.lines[t.cur.y].wrapped = true
		t.cur.x = 0
		t.lineFeed()
	}

	t.cur.pendingWrap = false

	if w == 2 && t.cur.x == t.cols-1 {
		if !t.modes.autowrap {
			return
		}

		t.screen.lines[t.cur.y].cells[t.cur.x] = blank(t.cur.pen.Bg)
		t.screen.lines[t.cur.y].wrapped = true
		t.cur.x = 0
		t.lineFeed()
	}

	cells := t.screen.lines[t.cur.y].cells

	if t.modes.insert {
		copy(cells[t.cur.x+w:], cells[t.cur.x:])
	}

	t.clearWide(cells, t.cur.x)

	if w == 2 {
		t.clearWide(cells, t.cur.x+1)
	}

	c := t.cur.pen
	c.Rune = r
	c.SetWidth(uint8(w))
	cells[t.cur.x] = c

	if w == 2 {
		c.Rune = 0
		c.SetWidth(0)
		cells[t.cur.x+1] = c
	}

	t.lastRune = r
	t.cur.x += w

	if t.cur.x >= t.cols {
		t.cur.x = t.cols - 1
		t.cur.pendingWrap = true
	}
}

func (t *Terminal) PrintASCII(s []byte) {
	if t.cur.charsets[t.cur.gl] == charsetDECGraphics || t.modes.insert {
		for _, b := range s {
			t.Print(rune(b))
		}

		return
	}

	for len(s) > 0 {
		x := t.cur.x

		if t.cur.pendingWrap || x >= t.cols-1 {
			t.Print(rune(s[0]))
			s = s[1:]
			continue
		}

		n := min(len(s), t.cols-1-x)
		cells := t.screen.lines[t.cur.y].cells
		t.clearWide(cells, x)

		if cells[x+n-1].Width() == 2 && x+n < len(cells) {
			cells[x+n] = blank(cells[x+n].Bg)
		}

		c := t.cur.pen
		c.SetWidth(1)

		for i, b := range s[:n] {
			c.Rune = rune(b)
			cells[x+i] = c
		}

		t.lastRune = rune(s[n-1])
		t.cur.x += n
		s = s[n:]
	}
}

func (t *Terminal) clearWide(cells []Cell, x int) {
	if x >= len(cells) {
		return
	}

	switch {
	case cells[x].Width() == 2 && x+1 < len(cells):
		cells[x+1] = blank(cells[x+1].Bg)
	case cells[x].Width() == 0 && x > 0:
		cells[x-1] = blank(cells[x-1].Bg)
	}
}

func (t *Terminal) Execute(b byte) {
	switch b {
	case 0x07:
		if t.onBell != nil {
			t.onBell()
		}
	case 0x08:
		t.cur.pendingWrap = false
		t.cur.x = max(t.cur.x-1, 0)
	case 0x09:
		t.tabForward(1)
	case 0x0a, 0x0b, 0x0c:
		t.lineFeed()

		if t.modes.newline {
			t.cur.x = 0
		}
	case 0x0d:
		t.cur.pendingWrap = false
		t.cur.x = 0
	case 0x0e:
		t.cur.gl = 1
	case 0x0f:
		t.cur.gl = 0
	}
}

func (t *Terminal) ESC(inter []byte, final byte) {
	if len(inter) == 1 {
		t.escIntermediate(inter[0], final)
		return
	}

	switch final {
	case '7':
		t.saveCursor()
	case '8':
		t.restoreCursor()
	case 'D':
		t.lineFeed()
	case 'E':
		t.lineFeed()
		t.cur.x = 0
	case 'M':
		t.reverseIndex()
	case 'H':
		t.tabs[t.cur.x] = true
	case 'c':
		t.reset()
	case '=':
		t.modes.keypadApp = true
	case '>':
		t.modes.keypadApp = false
	}
}

func (t *Terminal) escIntermediate(inter, final byte) {
	switch inter {
	case '(', ')':
		g := 0

		if inter == ')' {
			g = 1
		}

		t.cur.charsets[g] = charsetASCII

		if final == '0' {
			t.cur.charsets[g] = charsetDECGraphics
		}
	case '#':
		if final == '8' {
			t.screenAlignment()
		}
	}
}

func (t *Terminal) OSC(data []byte) {
	num, rest, ok := bytes.Cut(data, []byte{';'})

	if !ok {
		return
	}

	code, err := strconv.Atoi(string(num))

	if err != nil {
		return
	}

	switch code {
	case oscTitleIcon, oscTitle:
		t.title = strings.ToValidUTF8(string(rest[:min(len(rest), maxTitleLen)]), "")

		if t.onTitle != nil {
			t.onTitle(t.title)
		}
	case oscCwd:
		t.setCwd(string(rest))
	case oscLink:
		t.setLink(rest)
	case oscClipboard:
		t.setClipboard(rest)
	}
}

func (t *Terminal) lineFeed() {
	t.cur.pendingWrap = false

	if t.cur.y == t.bot {
		t.scrollUp(1)
		return
	}

	if t.cur.y < t.rows-1 {
		t.cur.y++
	}
}

func (t *Terminal) reverseIndex() {
	t.cur.pendingWrap = false

	if t.cur.y == t.top {
		t.scrollDown(1)
		return
	}

	if t.cur.y > 0 {
		t.cur.y--
	}
}

func (t *Terminal) scrollUp(n int) {
	t.scrollRegionUp(t.top, n, t.screen == t.primary && t.top == 0)
}

func (t *Terminal) scrollDown(n int) {
	t.scrollRegionDown(t.top, n)
}

func (t *Terminal) scrollRegionUp(top, n int, toScrollback bool) {
	n = min(n, t.bot-top+1)
	lines := t.screen.lines

	for range n {
		gone := lines[top]
		copy(lines[top:t.bot], lines[top+1:t.bot+1])

		fresh := gone

		if toScrollback {
			fresh = t.sb.push(gone)
		}

		fresh.reset(t.cols, t.cur.pen.Bg)
		lines[t.bot] = fresh
	}
}

func (t *Terminal) scrollRegionDown(top, n int) {
	n = min(n, t.bot-top+1)
	lines := t.screen.lines

	for range n {
		gone := lines[t.bot]
		copy(lines[top+1:t.bot+1], lines[top:t.bot])
		gone.reset(t.cols, t.cur.pen.Bg)
		lines[top] = gone
	}
}

func (t *Terminal) tabForward(n int) {
	t.cur.pendingWrap = false

	for ; n > 0 && t.cur.x < t.cols-1; n-- {
		t.cur.x++

		for t.cur.x < t.cols-1 && !t.tabs[t.cur.x] {
			t.cur.x++
		}
	}
}

func (t *Terminal) tabBackward(n int) {
	t.cur.pendingWrap = false

	for ; n > 0 && t.cur.x > 0; n-- {
		t.cur.x--

		for t.cur.x > 0 && !t.tabs[t.cur.x] {
			t.cur.x--
		}
	}
}

func (t *Terminal) savedIndex() int {
	if t.screen == t.alt {
		return 1
	}

	return 0
}

func (t *Terminal) saveCursor() {
	t.saved[t.savedIndex()] = t.cur
}

func (t *Terminal) restoreCursor() {
	t.cur = t.saved[t.savedIndex()]
	t.cur.x = min(t.cur.x, t.cols-1)
	t.cur.y = min(t.cur.y, t.rows-1)
}

func (t *Terminal) screenAlignment() {
	for i := range t.screen.lines {
		for x := range t.screen.lines[i].cells {
			t.screen.lines[i].cells[x] = Cell{Rune: 'E', Attr: 1 << attrWidthShift}
		}
	}

	t.top, t.bot = 0, t.rows-1
	t.cur.x, t.cur.y = 0, 0
}

func (t *Terminal) setAltScreen(on, saveCursor, clear bool) {
	if on == (t.screen == t.alt) {
		return
	}

	if on {
		if saveCursor {
			t.saved[0] = t.cur
		}

		t.screen = t.alt

		if clear {
			for i := range t.alt.lines {
				t.alt.lines[i].reset(t.cols, DefaultColor)
			}
		}

		return
	}

	t.screen = t.primary

	if saveCursor {
		t.cur = t.saved[0]
		t.cur.x = min(t.cur.x, t.cols-1)
		t.cur.y = min(t.cur.y, t.rows-1)
	}
}
