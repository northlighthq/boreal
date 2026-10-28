package vt

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/northlighthq/vt/parser"
)

const (
	defaultScrollback = 10_000
	tabWidth          = 8
	writeChunk        = 16 * 1024
)

type Option func(*Terminal)

func WithScrollback(n int) Option {
	return func(t *Terminal) {
		t.sb.max = max(n, 0)
	}
}

func WithReply(fn func([]byte)) Option {
	return func(t *Terminal) {
		t.reply = fn
	}
}

func WithTitle(fn func(string)) Option {
	return func(t *Terminal) {
		t.onTitle = fn
	}
}

func WithBell(fn func()) Option {
	return func(t *Terminal) {
		t.onBell = fn
	}
}

type modes struct {
	autowrap       bool
	insert         bool
	newline        bool
	origin         bool
	cursorKeysApp  bool
	keypadApp      bool
	bracketedPaste bool
	focusEvents    bool
	mouse          MouseMode
	mouseSGR       bool
}

type MouseMode uint8

const (
	MouseNone MouseMode = iota
	MouseClick
	MouseDrag
	MouseMotion
)

type cursor struct {
	x, y        int
	pendingWrap bool
	pen         Cell
	charsets    [2]charset
	gl          int
}

type Terminal struct {
	mu     sync.Mutex
	parser *parser.Parser
	gen    atomic.Uint64

	cols, rows int
	primary    *buffer
	alt        *buffer
	screen     *buffer
	sb         scrollback

	cur        cursor
	saved      [2]cursor
	top, bot   int
	tabs       []bool
	modes      modes
	cursorShow bool
	cursorSty  CursorStyle
	cursorBlnk bool
	cursorSet  bool
	lastRune   rune
	title      string
	widths     map[rune]uint8

	reply   func([]byte)
	onTitle func(string)

	links       []string
	linkIDs     map[string]uint16
	cwdHost     string
	cwdPath     string
	onClipboard func(string)
	onBell      func()
}

func New(cols, rows int, opts ...Option) *Terminal {
	cols, rows = max(cols, 1), max(rows, 1)

	t := &Terminal{
		cols:  cols,
		rows:  rows,
		sb:    scrollback{max: defaultScrollback},
		reply: func([]byte) {},
	}

	for _, opt := range opts {
		opt(t)
	}

	t.parser = parser.New(t)
	t.reset()
	return t
}

func (t *Terminal) reset() {
	t.primary = newBuffer(t.cols, t.rows)
	t.alt = newBuffer(t.cols, t.rows)
	t.screen = t.primary
	t.cur = cursor{}
	t.saved = [2]cursor{}
	t.top, t.bot = 0, t.rows-1
	t.modes = modes{autowrap: true}
	t.cursorShow = true
	t.cursorSty = CursorBlock
	t.cursorBlnk = false
	t.cursorSet = false
	t.resetTabs()
}

func (t *Terminal) resetTabs() {
	t.tabs = make([]bool, t.cols)

	for i := tabWidth; i < t.cols; i += tabWidth {
		t.tabs[i] = true
	}
}

func (t *Terminal) Write(p []byte) (int, error) {
	for rest := p; len(rest) > 0; {
		n := min(len(rest), writeChunk)
		err := t.feed(rest[:n])
		t.gen.Add(1)

		if err != nil {
			return len(p) - len(rest), err
		}

		rest = rest[n:]
	}

	return len(p), nil
}

func (t *Terminal) feed(p []byte) (err error) {
	t.mu.Lock()

	defer func() {
		if r := recover(); r != nil {
			t.parser = parser.New(t)
			t.reset()
			err = fmt.Errorf("vt: %v", r)
		}

		t.mu.Unlock()
	}()

	t.parser.Feed(p)
	return nil
}

func (t *Terminal) Generation() uint64 {
	return t.gen.Load()
}

func (t *Terminal) Size() (cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.cols, t.rows
}

func (t *Terminal) Resize(cols, rows int) {
	cols, rows = max(cols, 1), max(rows, 1)

	t.mu.Lock()
	defer t.mu.Unlock()

	if cols == t.cols && rows == t.rows {
		return
	}

	if cols != t.cols {
		cur := &t.cur

		if t.screen == t.alt {
			cur = &t.saved[0]
		}

		t.reflow(cols, rows, cur)
		t.cols = cols
	} else {
		t.resizeBuffer(t.primary, cols, rows, true)
	}

	t.resizeBuffer(t.alt, cols, rows, false)

	t.cols, t.rows = cols, rows
	t.top, t.bot = 0, rows-1
	t.resetTabs()

	t.cur.x = min(t.cur.x, cols-1)
	t.cur.y = min(t.cur.y, rows-1)
	t.cur.pendingWrap = false

	for i := range t.saved {
		t.saved[i].x = min(t.saved[i].x, cols-1)
		t.saved[i].y = min(t.saved[i].y, rows-1)
	}

	t.gen.Add(1)
}

func (t *Terminal) resizeBuffer(b *buffer, cols, rows int, isPrimary bool) {
	active := b == t.screen

	for i := range b.lines {
		b.lines[i].resize(cols)
	}

	if rows < len(b.lines) {
		shift := 0

		if active {
			shift = max(t.cur.y-rows+1, 0)
		}

		for i := 0; i < shift; i++ {
			if isPrimary {
				t.sb.push(b.lines[i])
			}
		}

		b.lines = b.lines[shift : shift+rows]

		if active {
			t.cur.y -= shift
		}

		return
	}

	for len(b.lines) < rows {
		if isPrimary && active {
			if l, ok := t.sb.pop(); ok {
				l.resize(cols)
				b.lines = append([]line{l}, b.lines...)
				t.cur.y++
				continue
			}
		}

		b.lines = append(b.lines, newLine(cols, DefaultColor))
	}
}

type Frame struct {
	Cols, Rows int
	Cells      []Cell
	Cursor     Cursor
	Title      string
	Scrollback int
	Offset     int
	TopLine    int
	AltScreen  bool
}

func (f *Frame) Row(y int) []Cell {
	return f.Cells[y*f.Cols : (y+1)*f.Cols]
}

func (t *Terminal) Snapshot(f *Frame, offset int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	sbLen := 0

	if t.screen == t.primary {
		sbLen = t.sb.len()
	}

	offset = min(max(offset, 0), sbLen)

	f.Cols, f.Rows = t.cols, t.rows
	f.Title = t.title
	f.Scrollback = sbLen
	f.Offset = offset
	f.TopLine = t.sb.dropped + sbLen - offset
	f.AltScreen = t.screen == t.alt

	n := t.cols * t.rows

	if cap(f.Cells) < n {
		f.Cells = make([]Cell, n)
	}

	f.Cells = f.Cells[:n]

	for y := 0; y < t.rows; y++ {
		var src []Cell
		idx := y - offset

		if idx < 0 {
			src = t.sb.at(sbLen + idx).cells
		} else {
			src = t.screen.lines[idx].cells
		}

		dst := f.Row(y)
		c := copy(dst, src)

		for x := c; x < len(dst); x++ {
			dst[x] = blank(DefaultColor)
		}
	}

	f.Cursor = Cursor{
		X:       t.cur.x,
		Y:       t.cur.y + offset,
		Visible: t.cursorShow && t.cur.y+offset < t.rows,
		Style:   t.cursorSty,
		Blink:   t.cursorBlnk,
		Custom:  t.cursorSet,
	}
}

func (t *Terminal) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.title
}

type Modes struct {
	CursorKeysApp  bool
	KeypadApp      bool
	BracketedPaste bool
	FocusEvents    bool
	Mouse          MouseMode
	MouseSGR       bool
	AltScreen      bool
}

func (t *Terminal) Modes() Modes {
	t.mu.Lock()
	defer t.mu.Unlock()

	return Modes{
		CursorKeysApp:  t.modes.cursorKeysApp,
		KeypadApp:      t.modes.keypadApp,
		BracketedPaste: t.modes.bracketedPaste,
		FocusEvents:    t.modes.focusEvents,
		Mouse:          t.modes.mouse,
		MouseSGR:       t.modes.mouseSGR,
		AltScreen:      t.screen == t.alt,
	}
}
