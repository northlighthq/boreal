package vt

import (
	"fmt"

	"github.com/northlighthq/vt/parser"
)

const (
	decCKM          = 1
	decOM           = 6
	decAWM          = 7
	decTCEM         = 25
	decAltScreen    = 47
	mouseX10        = 9
	mouseClick      = 1000
	mouseDrag       = 1002
	mouseMotion     = 1003
	focusEvents     = 1004
	mouseSGR        = 1006
	decAltScreen2   = 1047
	decSaveCursor   = 1048
	decAltScreenSav = 1049
	bracketedPaste  = 2004
)

const (
	ansiIRM = 4
	ansiLNM = 20
)

func (t *Terminal) CSI(p *parser.Params, inter []byte, final byte) {
	if len(inter) > 0 {
		t.csiPrefixed(p, inter, final)
		return
	}

	if final != 'b' {
		defer func() { t.lastRune = 0 }()
	}

	switch final {
	case '@':
		t.insertChars(p.Get(0, 1))
	case 'A':
		t.cursorUp(p.Get(0, 1))
	case 'B', 'e':
		t.cursorDown(p.Get(0, 1))
	case 'C', 'a':
		t.moveTo(t.cur.x+p.Get(0, 1), t.cur.y)
	case 'D':
		t.moveTo(t.cur.x-p.Get(0, 1), t.cur.y)
	case 'E':
		t.cursorDown(p.Get(0, 1))
		t.cur.x = 0
	case 'F':
		t.cursorUp(p.Get(0, 1))
		t.cur.x = 0
	case 'G', '`':
		t.moveTo(p.Get(0, 1)-1, t.cur.y)
	case 'H', 'f':
		t.cursorPosition(p.Get(0, 1)-1, p.Get(1, 1)-1)
	case 'I':
		t.tabForward(p.Get(0, 1))
	case 'J':
		t.eraseDisplay(p.Raw(0))
	case 'K':
		t.eraseLine(p.Raw(0))
	case 'L':
		t.insertLines(p.Get(0, 1))
	case 'M':
		t.deleteLines(p.Get(0, 1))
	case 'P':
		t.deleteChars(p.Get(0, 1))
	case 'S':
		t.scrollUp(p.Get(0, 1))
	case 'T':
		t.scrollDown(p.Get(0, 1))
	case 'X':
		t.eraseChars(p.Get(0, 1))
	case 'Z':
		t.tabBackward(p.Get(0, 1))
	case 'b':
		t.repeat(p.Get(0, 1))
	case 'c':
		t.reply([]byte("\x1b[?62;22c"))
	case 'd':
		t.cursorPosition(p.Get(0, 1)-1, t.cur.x)
	case 'g':
		t.clearTabs(p.Raw(0))
	case 'h', 'l':
		for i := 0; i < max(p.Len(), 1); i++ {
			t.setANSIMode(p.Raw(i), final == 'h')
		}
	case 'm':
		t.sgr(p)
	case 'n':
		t.deviceStatus(p.Raw(0))
	case 'r':
		t.setScrollRegion(p.Get(0, 1)-1, p.Get(1, t.rows)-1)
	case 's':
		t.saveCursor()
	case 'u':
		t.restoreCursor()
	}
}

func (t *Terminal) csiPrefixed(p *parser.Params, inter []byte, final byte) {
	switch {
	case inter[0] == '?' && (final == 'h' || final == 'l'):
		for i := 0; i < p.Len(); i++ {
			t.setDECMode(p.Raw(i), final == 'h')
		}
	case inter[0] == '>' && final == 'c':
		t.reply([]byte("\x1b[>0;10;1c"))
	case inter[0] == ' ' && final == 'q':
		t.setCursorStyle(p.Raw(0))
	case inter[0] == '!' && final == 'p':
		t.softReset()
	}
}

func (t *Terminal) moveTo(x, y int) {
	t.cur.pendingWrap = false
	t.cur.x = min(max(x, 0), t.cols-1)
	t.cur.y = min(max(y, 0), t.rows-1)
}

func (t *Terminal) cursorUp(n int) {
	limit := 0

	if t.cur.y >= t.top {
		limit = t.top
	}

	t.moveTo(t.cur.x, max(t.cur.y-n, limit))
}

func (t *Terminal) cursorDown(n int) {
	limit := t.rows - 1

	if t.cur.y <= t.bot {
		limit = t.bot
	}

	t.moveTo(t.cur.x, min(t.cur.y+n, limit))
}

func (t *Terminal) cursorPosition(row, col int) {
	if !t.modes.origin {
		t.moveTo(col, row)
		return
	}

	t.moveTo(col, min(row+t.top, t.bot))
}

func (t *Terminal) eraseDisplay(mode int) {
	bg := t.cur.pen.Bg
	lines := t.screen.lines

	switch mode {
	case 0:
		t.eraseLine(0)

		for y := t.cur.y + 1; y < t.rows; y++ {
			lines[y].reset(t.cols, bg)
		}
	case 1:
		t.eraseLine(1)

		for y := 0; y < t.cur.y; y++ {
			lines[y].reset(t.cols, bg)
		}
	case 2:
		for y := range lines {
			lines[y].reset(t.cols, bg)
		}
	case 3:
		t.sb.clear()
	}
}

func (t *Terminal) eraseLine(mode int) {
	t.cur.pendingWrap = false
	l := &t.screen.lines[t.cur.y]
	bg := t.cur.pen.Bg

	switch mode {
	case 0:
		l.clear(t.cur.x, t.cols, bg)
		l.wrapped = false
	case 1:
		l.clear(0, t.cur.x+1, bg)
	case 2:
		l.clear(0, t.cols, bg)
		l.wrapped = false
	}
}

func (t *Terminal) eraseChars(n int) {
	t.cur.pendingWrap = false
	t.screen.lines[t.cur.y].clear(t.cur.x, t.cur.x+n, t.cur.pen.Bg)
}

func (t *Terminal) insertChars(n int) {
	t.cur.pendingWrap = false
	cells := t.screen.lines[t.cur.y].cells
	n = min(n, t.cols-t.cur.x)

	copy(cells[t.cur.x+n:], cells[t.cur.x:])
	t.screen.lines[t.cur.y].clear(t.cur.x, t.cur.x+n, t.cur.pen.Bg)
}

func (t *Terminal) deleteChars(n int) {
	t.cur.pendingWrap = false
	cells := t.screen.lines[t.cur.y].cells
	n = min(n, t.cols-t.cur.x)

	copy(cells[t.cur.x:], cells[t.cur.x+n:])
	t.screen.lines[t.cur.y].clear(t.cols-n, t.cols, t.cur.pen.Bg)
}

func (t *Terminal) insertLines(n int) {
	if t.cur.y < t.top || t.cur.y > t.bot {
		return
	}

	t.scrollRegionDown(t.cur.y, n)
	t.cur.x = 0
	t.cur.pendingWrap = false
}

func (t *Terminal) deleteLines(n int) {
	if t.cur.y < t.top || t.cur.y > t.bot {
		return
	}

	t.scrollRegionUp(t.cur.y, n, false)
	t.cur.x = 0
	t.cur.pendingWrap = false
}

func (t *Terminal) repeat(n int) {
	if t.lastRune == 0 {
		return
	}

	r := t.lastRune

	for range min(n, t.cols*t.rows) {
		t.Print(r)
	}
}

func (t *Terminal) clearTabs(mode int) {
	switch mode {
	case 0:
		t.tabs[t.cur.x] = false
	case 3:
		clear(t.tabs)
	}
}

func (t *Terminal) setScrollRegion(top, bot int) {
	bot = min(bot, t.rows-1)

	if top >= bot {
		return
	}

	t.top, t.bot = top, bot
	t.cursorPosition(0, 0)
}

func (t *Terminal) deviceStatus(mode int) {
	switch mode {
	case 5:
		t.reply([]byte("\x1b[0n"))
	case 6:
		y := t.cur.y

		if t.modes.origin {
			y -= t.top
		}

		t.reply(fmt.Appendf(nil, "\x1b[%d;%dR", y+1, t.cur.x+1))
	}
}

func (t *Terminal) setANSIMode(mode int, on bool) {
	switch mode {
	case ansiIRM:
		t.modes.insert = on
	case ansiLNM:
		t.modes.newline = on
	}
}

func (t *Terminal) setDECMode(mode int, on bool) {
	switch mode {
	case decCKM:
		t.modes.cursorKeysApp = on
	case decOM:
		t.modes.origin = on
		t.cursorPosition(0, 0)
	case decAWM:
		t.modes.autowrap = on
	case decTCEM:
		t.cursorShow = on
	case decAltScreen, decAltScreen2:
		t.setAltScreen(on, false, mode == decAltScreen2)
	case decSaveCursor:
		if on {
			t.saveCursor()
		} else {
			t.restoreCursor()
		}
	case decAltScreenSav:
		t.setAltScreen(on, true, true)
	case mouseX10, mouseClick:
		t.setMouse(MouseClick, on)
	case mouseDrag:
		t.setMouse(MouseDrag, on)
	case mouseMotion:
		t.setMouse(MouseMotion, on)
	case focusEvents:
		t.modes.focusEvents = on
	case mouseSGR:
		t.modes.mouseSGR = on
	case bracketedPaste:
		t.modes.bracketedPaste = on
	}
}

func (t *Terminal) setMouse(m MouseMode, on bool) {
	if on {
		t.modes.mouse = m
		return
	}

	if t.modes.mouse == m {
		t.modes.mouse = MouseNone
	}
}

func (t *Terminal) setCursorStyle(n int) {
	t.cursorSet = n != 0

	switch n {
	case 0, 1:
		t.cursorSty, t.cursorBlnk = CursorBlock, true
	case 2:
		t.cursorSty, t.cursorBlnk = CursorBlock, false
	case 3:
		t.cursorSty, t.cursorBlnk = CursorUnderline, true
	case 4:
		t.cursorSty, t.cursorBlnk = CursorUnderline, false
	case 5:
		t.cursorSty, t.cursorBlnk = CursorBar, true
	case 6:
		t.cursorSty, t.cursorBlnk = CursorBar, false
	}
}

func (t *Terminal) softReset() {
	t.cursorShow = true
	t.modes.insert = false
	t.modes.origin = false
	t.modes.autowrap = true
	t.modes.cursorKeysApp = false
	t.modes.keypadApp = false
	t.top, t.bot = 0, t.rows-1
	t.cur.pen = Cell{}
	t.cur.charsets = [2]charset{}
	t.cur.gl = 0
	t.saved = [2]cursor{}
}

func (t *Terminal) sgr(p *parser.Params) {
	pen := &t.cur.pen

	if p.Len() == 0 {
		*pen = Cell{Link: pen.Link}
		return
	}

	for i := 0; i < p.Len(); i++ {
		switch n := p.Raw(i); {
		case n == 0:
			*pen = Cell{Link: pen.Link}
		case n == 1:
			pen.Attr |= AttrBold
		case n == 2:
			pen.Attr |= AttrFaint
		case n == 3:
			pen.Attr |= AttrItalic
		case n == 4:
			pen.Attr &^= AttrDoubleUnderline | AttrCurlyUnderline
			pen.Attr |= underlineStyle(p, i)
			i += countSub(p, i)
		case n == 5 || n == 6:
			pen.Attr |= AttrBlink
		case n == 7:
			pen.Attr |= AttrInverse
		case n == 8:
			pen.Attr |= AttrInvisible
		case n == 9:
			pen.Attr |= AttrStrike
		case n == 21:
			pen.Attr |= AttrDoubleUnderline
		case n == 22:
			pen.Attr &^= AttrBold | AttrFaint
		case n == 23:
			pen.Attr &^= AttrItalic
		case n == 24:
			pen.Attr &^= AttrUnderline | AttrDoubleUnderline | AttrCurlyUnderline
		case n == 25:
			pen.Attr &^= AttrBlink
		case n == 27:
			pen.Attr &^= AttrInverse
		case n == 28:
			pen.Attr &^= AttrInvisible
		case n == 29:
			pen.Attr &^= AttrStrike
		case n >= 30 && n <= 37:
			pen.Fg = Indexed(uint8(n - 30))
		case n == 38:
			c, used := extendedColor(p, i)
			pen.Fg = c
			i += used
		case n == 39:
			pen.Fg = DefaultColor
		case n >= 40 && n <= 47:
			pen.Bg = Indexed(uint8(n - 40))
		case n == 48:
			c, used := extendedColor(p, i)
			pen.Bg = c
			i += used
		case n == 49:
			pen.Bg = DefaultColor
		case n == 58, n == 59:
			if n == 58 {
				_, used := extendedColor(p, i)
				i += used
			}
		case n >= 90 && n <= 97:
			pen.Fg = Indexed(uint8(n - 90 + 8))
		case n >= 100 && n <= 107:
			pen.Bg = Indexed(uint8(n - 100 + 8))
		}
	}
}

func countSub(p *parser.Params, i int) int {
	n := 0

	for p.IsSub(i + n + 1) {
		n++
	}

	return n
}

func underlineStyle(p *parser.Params, i int) Attr {
	if !p.IsSub(i + 1) {
		return AttrUnderline
	}

	switch p.Raw(i + 1) {
	case 0:
		return 0
	case 2:
		return AttrDoubleUnderline
	case 3:
		return AttrCurlyUnderline
	default:
		return AttrUnderline
	}
}

func extendedColor(p *parser.Params, i int) (Color, int) {
	if p.IsSub(i + 1) {
		used := countSub(p, i)

		switch p.Raw(i + 1) {
		case 5:
			return Indexed(uint8(p.Raw(i + 2))), used
		case 2:
			base := i + 2

			if used >= 5 {
				base = i + 3
			}

			return RGB(uint8(p.Raw(base)), uint8(p.Raw(base+1)), uint8(p.Raw(base+2))), used
		}

		return DefaultColor, used
	}

	switch p.Raw(i + 1) {
	case 5:
		return Indexed(uint8(p.Raw(i + 2))), 2
	case 2:
		return RGB(uint8(p.Raw(i+2)), uint8(p.Raw(i+3)), uint8(p.Raw(i+4))), 4
	}

	return DefaultColor, 1
}
