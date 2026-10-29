package termview

import (
	"image"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/northlighthq/glyphs"
	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const (
	minTextSize  = unit.Sp(8)
	maxTextSize  = unit.Sp(40)
	padding      = unit.Dp(12)
	blinkTimeout = 30 * time.Second
	resizeDelay  = 100 * time.Millisecond
)

type Backend interface {
	Terminal() *vt.Terminal
	Send(p []byte)
	Resize(cols, rows int)
}

const (
	ActionSplitRight = "split-right"
	ActionSplitDown  = "split-down"
	ActionClosePane  = "close-pane"
)

type Extra struct {
	Item   kit.MenuItem
	Action string
}

type Settings struct {
	TextSize     unit.Sp
	LineHeight   float32
	CursorStyle  vt.CursorStyle
	CursorBlink  bool
	CopyOnSelect bool
	Ligatures    bool
	Highlight    bool
}

type metrics struct {
	px       int
	line     float32
	w, h     int
	baseline int
}

type View struct {
	backend Backend
	kit     *kit.Kit
	theme   *theme.Theme
	fonts   *glyphs.Set
	glyphs  *glyphs.Cache

	set     Settings
	hlCells []vt.Cell
	hlOffs  []int
	hlAlt   bool
	m       metrics
	cols    int
	rows    int
	gridOff image.Point

	frame     vt.Frame
	modes     vt.Modes
	offset    int
	snapGen   uint64
	snapped   bool
	pending   image.Point
	pendingAt time.Time
	cache     rowCache
	cursorImg cursorImage
	scrollY   float32

	focused    bool
	wantFocus  bool
	lastInput  time.Time
	blinkStart time.Time

	sel    selection
	search search
	menu   kit.Menu
	hover  hoverLink

	OnAction  func(action string)
	OnOpenURL func(url string)
	OnFocus   func()
	Extras    func() []Extra
	Expand    func(word string) (string, bool)
	extras    []Extra
	word      []rune

	Debug     bool
	lastFrame time.Duration
}

func New(b Backend, k *kit.Kit, fonts *glyphs.Set, s Settings) *View {
	v := &View{
		backend:   b,
		kit:       k,
		theme:     k.Th,
		fonts:     fonts,
		wantFocus: true,
	}
	v.SetSettings(s)
	return v
}

func (v *View) SetSettings(s Settings) {
	if s.Highlight != v.set.Highlight {
		v.cache.invalidate()
	}

	if s.TextSize == 0 {
		s.TextSize = 13
	}

	if s.LineHeight == 0 {
		s.LineHeight = 1.2
	}

	v.set = s
	v.m.px = 0
}

func (v *View) SetTheme(th *theme.Theme) {
	v.theme = th
	v.cache.invalidate()
	v.cursorImg.img = nil
}

func (v *View) Focus() {
	v.wantFocus = true
}

func (v *View) Focused() bool {
	return v.focused
}

func (v *View) Title() string {
	return v.frame.Title
}

func (v *View) Size() (int, int) {
	return v.cols, v.rows
}

func (v *View) SendText(s string) {
	v.send([]byte(s))
}

func (v *View) OpenSearch() {
	v.search.show()
}

func (v *View) Layout(gtx layout.Context) layout.Dimensions {
	start := time.Now()
	size := gtx.Constraints.Max

	term := v.backend.Terminal()
	v.updateMetrics(gtx)
	v.updateGrid(gtx, size)
	gen := term.Generation()

	if !v.snapped || gen != v.snapGen {
		v.modes = term.Modes()
	}

	v.handleEvents(gtx)
	v.handleMenu(gtx)

	if v.wantFocus {
		gtx.Execute(key.FocusCmd{Tag: v})
		v.wantFocus = false
	}

	fresh := !v.snapped || gen != v.snapGen || v.offset != v.frame.Offset

	if fresh {
		term.Snapshot(&v.frame, v.offset)
		v.offset = v.frame.Offset
		v.snapGen, v.snapped = gen, true
	}

	v.search.refresh(v)

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	paint.ColorOp{Color: v.theme.Background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	event.Op(gtx.Ops, v)
	key.InputHintOp{Tag: v, Hint: key.HintAny}.Add(gtx.Ops)

	cursor := pointer.CursorText

	if v.hover.active {
		cursor = pointer.CursorPointer
	}

	cursor.Add(gtx.Ops)

	grid := op.Offset(v.gridOff).Push(gtx.Ops)
	v.paintSelection(gtx)
	v.paintSearch(gtx)
	v.paintRows(gtx, fresh)
	v.paintLink(gtx)
	v.paintCursor(gtx)
	grid.Pop()
	v.paintScrollbar(gtx, size)

	if v.search.open {
		v.layoutSearchBar(gtx, size)
	}

	if v.Debug {
		v.paintDebug(gtx, size)
	}

	v.kit.Menu(gtx, &v.menu)
	area.Pop()

	v.scheduleBlink(gtx)
	v.lastFrame = time.Since(start)
	return layout.Dimensions{Size: size}
}

func (v *View) updateMetrics(gtx layout.Context) {
	px := gtx.Sp(v.set.TextSize)

	if px == v.m.px && v.set.LineHeight == v.m.line && v.glyphs != nil {
		return
	}

	v.glyphs = sharedCache(v.fonts, px, v.set.LineHeight)
	gm := v.glyphs.Metrics()
	v.m = metrics{px: px, line: v.set.LineHeight, w: gm.CellW, h: gm.CellH, baseline: gm.Baseline}
	v.cache.invalidate()
	v.cursorImg.img = nil
}

func (v *View) updateGrid(gtx layout.Context, size image.Point) {
	pad := gtx.Dp(padding)
	cols := max((size.X-2*pad)/v.m.w, 1)
	rows := max((size.Y-2*pad)/v.m.h, 1)
	v.gridOff = image.Pt(pad, pad)

	if cols == v.cols && rows == v.rows {
		v.pendingAt = time.Time{}
		return
	}

	if v.cols != 0 {
		if want := image.Pt(cols, rows); want != v.pending || v.pendingAt.IsZero() {
			v.pending, v.pendingAt = want, gtx.Now
		}

		if due := v.pendingAt.Add(resizeDelay); gtx.Now.Before(due) {
			gtx.Execute(op.InvalidateCmd{At: due})
			return
		}
	}

	v.pendingAt = time.Time{}
	v.cols, v.rows = cols, rows
	v.backend.Resize(cols, rows)
}

func (v *View) zoom(delta unit.Sp) {
	v.set.TextSize = min(max(v.set.TextSize+delta, minTextSize), maxTextSize)
	v.m.px = 0
}

const (
	blinkVisible = 520 * time.Millisecond
	blinkFadeOut = 180 * time.Millisecond
	blinkHidden  = 300 * time.Millisecond
	blinkFadeIn  = 180 * time.Millisecond
	blinkCycle   = blinkVisible + blinkFadeOut + blinkHidden + blinkFadeIn
	fadeFrame    = 33 * time.Millisecond
)

func (v *View) cursorAlpha() float32 {
	if !v.blinking() {
		return 1
	}

	p := time.Since(v.blinkStart) % blinkCycle

	switch {
	case p < blinkVisible:
		return 1
	case p < blinkVisible+blinkFadeOut:
		return 1 - smooth(float32(p-blinkVisible)/float32(blinkFadeOut))
	case p < blinkVisible+blinkFadeOut+blinkHidden:
		return 0
	default:
		return smooth(float32(p-blinkVisible-blinkFadeOut-blinkHidden) / float32(blinkFadeIn))
	}
}

func smooth(t float32) float32 {
	return t * t * (3 - 2*t)
}

func (v *View) blinking() bool {
	blink := v.set.CursorBlink

	if v.frame.Cursor.Custom {
		blink = v.frame.Cursor.Blink
	}

	return blink && v.focused && time.Since(v.lastInput) < blinkTimeout
}

func (v *View) scheduleBlink(gtx layout.Context) {
	if !v.blinking() {
		return
	}

	p := time.Since(v.blinkStart) % blinkCycle
	var wait time.Duration

	switch {
	case p < blinkVisible:
		wait = blinkVisible - p
	case p < blinkVisible+blinkFadeOut:
		wait = fadeFrame
	case p < blinkVisible+blinkFadeOut+blinkHidden:
		wait = blinkVisible + blinkFadeOut + blinkHidden - p
	default:
		wait = fadeFrame
	}

	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(wait)})
}

func (v *View) touchInput() {
	v.lastInput = time.Now()
	v.blinkStart = v.lastInput
}

func (v *View) SetFonts(f *glyphs.Set) {
	if f == nil || f == v.fonts {
		return
	}

	v.fonts = f
	v.glyphs = nil
}

func (v *View) LastInput() time.Time {
	return v.lastInput
}
