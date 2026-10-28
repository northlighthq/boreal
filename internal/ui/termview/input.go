package termview

import (
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"github.com/northlighthq/vt"
	"github.com/northlighthq/vt/input"

	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	mimeText       = "application/text"
	scrollMax      = 1 << 20
	altScrollLines = 3
	multiClick     = 450 * time.Millisecond
)

const allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper

var specialKeys = map[key.Name]input.Key{
	key.NameReturn:         input.KeyEnter,
	key.NameEnter:          input.KeyEnter,
	key.NameTab:            input.KeyTab,
	key.NameDeleteBackward: input.KeyBackspace,
	key.NameEscape:         input.KeyEscape,
	key.NameUpArrow:        input.KeyUp,
	key.NameDownArrow:      input.KeyDown,
	key.NameRightArrow:     input.KeyRight,
	key.NameLeftArrow:      input.KeyLeft,
	key.NameHome:           input.KeyHome,
	key.NameEnd:            input.KeyEnd,
	key.NamePageUp:         input.KeyPageUp,
	key.NamePageDown:       input.KeyPageDown,
	key.NameDeleteForward:  input.KeyDelete,
	"Insert":               input.KeyInsert,
	key.NameF1:             input.KeyF1,
	key.NameF2:             input.KeyF2,
	key.NameF3:             input.KeyF3,
	key.NameF4:             input.KeyF4,
	key.NameF5:             input.KeyF5,
	key.NameF6:             input.KeyF6,
	key.NameF7:             input.KeyF7,
	key.NameF8:             input.KeyF8,
	key.NameF9:             input.KeyF9,
	key.NameF10:            input.KeyF10,
	key.NameF11:            input.KeyF11,
	key.NameF12:            input.KeyF12,
}

const (
	menuCopy = iota
	menuPaste
	menuSelectAll
	menuSep1
	menuFind
	menuClear
	menuSep2
	menuSplitRight
	menuSplitDown
	menuSep3
	menuClose
	menuExtras
)

func (v *View) filters() []event.Filter {
	return []event.Filter{
		key.FocusFilter{Target: v},
		transfer.TargetFilter{Target: v, Type: mimeText},
		pointer.Filter{
			Target:  v,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: -scrollMax, Max: scrollMax},
		},
		key.Filter{Focus: v, Name: key.NameTab, Optional: allMods},
		key.Filter{Focus: v, Name: "", Optional: allMods},
	}
}

func (v *View) handleEvents(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(v.filters()...)

		if !ok {
			return
		}

		switch e := ev.(type) {
		case key.FocusEvent:
			v.focused = e.Focus
			v.touchInput()

			if e.Focus && v.OnFocus != nil {
				v.OnFocus()
			}

			v.backend.Send(input.EncodeFocus(e.Focus, v.modes))
		case key.EditEvent:
			v.typeText(e.Text)
		case key.Event:
			if e.State == key.Press {
				v.handleKey(gtx, e)
			}

			v.updateHoverMods(e.Modifiers)
		case pointer.Event:
			v.handlePointer(gtx, e)
		case transfer.DataEvent:
			v.paste(e)
		}
	}
}

func (v *View) handleKey(gtx layout.Context, e key.Event) {
	if v.shortcut(gtx, e) {
		return
	}

	if v.trackKey(e) {
		return
	}

	mods := toMod(e.Modifiers)

	if k, ok := specialKeys[e.Name]; ok {
		v.send(input.EncodeKey(k, mods, v.modes))
		return
	}

	ctrl := e.Modifiers.Contain(key.ModCtrl)
	alt := e.Modifiers.Contain(key.ModAlt)

	if ctrl == alt {
		return
	}

	r, ok := keyRune(e)

	if !ok {
		return
	}

	v.send(input.EncodeRune(r, mods))
}

func (v *View) shortcut(gtx layout.Context, e key.Event) bool {
	m := e.Modifiers
	ctrl := m.Contain(key.ModCtrl)
	shift := m.Contain(key.ModShift)
	ctrlShift := ctrl && shift

	switch {
	case ctrlShift && e.Name == "C":
		v.copySelection(gtx)
	case ctrl && !shift && e.Name == "C" && v.sel.active:
		v.copySelection(gtx)
		v.sel.clear()
	case ctrlShift && e.Name == "V", shift && e.Name == "Insert":
		gtx.Execute(clipboard.ReadCmd{Tag: v})
	case ctrlShift && e.Name == "F":
		v.search.show()
	case ctrlShift && e.Name == "A":
		v.selectAll()
	case shift && !ctrl && e.Name == key.NamePageUp:
		v.offset += v.rows - 1
	case shift && !ctrl && e.Name == key.NamePageDown:
		v.offset = max(v.offset-(v.rows-1), 0)
	case ctrlShift && e.Name == key.NameHome:
		v.offset = v.frame.Scrollback
	case ctrlShift && e.Name == key.NameEnd:
		v.offset = 0
	case ctrl && (e.Name == "+" || e.Name == "="):
		v.zoom(1)
	case ctrl && e.Name == "-":
		v.zoom(-1)
	case ctrl && e.Name == "0":
		v.set.TextSize = 13
		v.m.px = 0
	case ctrlShift && e.Name == key.NameF12:
		v.Debug = !v.Debug
	default:
		return false
	}

	return true
}

func keyRune(e key.Event) (rune, bool) {
	name := string(e.Name)

	if e.Name == key.NameSpace {
		return ' ', true
	}

	rs := []rune(name)

	if len(rs) != 1 {
		return 0, false
	}

	r := rs[0]

	if !e.Modifiers.Contain(key.ModShift) {
		r = []rune(strings.ToLower(name))[0]
	}

	return r, true
}

func toMod(m key.Modifiers) input.Mod {
	var out input.Mod

	if m.Contain(key.ModShift) {
		out |= input.ModShift
	}

	if m.Contain(key.ModAlt) {
		out |= input.ModAlt
	}

	if m.Contain(key.ModCtrl) {
		out |= input.ModCtrl
	}

	return out
}

func (v *View) cellAt(p f32.Point) (x, y int) {
	px := int(p.X) - v.gridOff.X
	py := int(p.Y) - v.gridOff.Y
	x = min(max(px/v.m.w, 0), max(v.frame.Cols-1, 0))
	y = py / v.m.h

	if py < 0 {
		y = -1
	}

	return x, y
}

func (v *View) posAt(p f32.Point) vt.Pos {
	x, y := v.cellAt(p)
	y = min(max(y, 0), v.frame.Rows-1)
	return vt.Pos{Line: v.frame.TopLine + y, Col: x}
}

func (v *View) reporting(m key.Modifiers) bool {
	return v.modes.Mouse != vt.MouseNone && !m.Contain(key.ModShift)
}

func (v *View) handlePointer(gtx layout.Context, e pointer.Event) {
	switch e.Kind {
	case pointer.Press:
		v.wantFocus = true
		v.touchInput()

		if e.Buttons.Contain(pointer.ButtonSecondary) && !v.reporting(e.Modifiers) {
			v.openMenu(e.Position.Round())
			return
		}

		if v.reporting(e.Modifiers) {
			v.reportMouse(e, input.MousePress)
			return
		}

		if e.Modifiers.Contain(key.ModCtrl) && v.hover.active {
			if v.OnOpenURL != nil {
				v.OnOpenURL(v.hover.url)
			}

			return
		}

		if e.Buttons.Contain(pointer.ButtonPrimary) {
			v.pressSelection(e)
		}

	case pointer.Drag:
		if v.reporting(e.Modifiers) {
			v.reportMouse(e, input.MouseMove)
			return
		}

		v.dragSelection(e)

	case pointer.Release:
		if v.reporting(e.Modifiers) {
			v.reportMouse(e, input.MouseUp)
			return
		}

		if v.sel.dragging {
			v.sel.dragging = false

			if v.set.CopyOnSelect && v.sel.active {
				v.copySelection(gtx)
			}
		}

	case pointer.Move:
		v.updateHover(e.Position, e.Modifiers)

		if v.modes.Mouse == vt.MouseMotion && !e.Modifiers.Contain(key.ModShift) {
			v.reportMouse(e, input.MouseMove)
		}

	case pointer.Leave:
		v.hover = hoverLink{}

	case pointer.Scroll:
		v.scroll(e)
	}
}

func (v *View) reportMouse(e pointer.Event, action input.MouseAction) {
	x, y := v.cellAt(e.Position)

	if y < 0 || y >= v.frame.Rows {
		return
	}

	b := input.MouseLeft

	switch {
	case e.Buttons.Contain(pointer.ButtonSecondary):
		b = input.MouseRight
	case e.Buttons.Contain(pointer.ButtonTertiary):
		b = input.MouseMiddle
	case action == input.MouseMove && e.Buttons == 0:
		b = input.MouseRelease
	}

	v.backend.Send(input.EncodeMouse(b, action, x, y, toMod(e.Modifiers), v.modes))
}

func (v *View) scroll(e pointer.Event) {
	v.scrollY += e.Scroll.Y
	lines := int(v.scrollY / float32(v.m.h))

	if lines == 0 {
		return
	}

	v.scrollY -= float32(lines * v.m.h)

	if !v.modes.AltScreen && !v.reporting(e.Modifiers) {
		v.offset = max(v.offset-lines, 0)
		return
	}

	up := lines < 0
	n := min(abs(lines), 10)

	if v.reporting(e.Modifiers) {
		x, y := v.cellAt(e.Position)
		b := input.MouseButton(input.MouseWheelDown)

		if up {
			b = input.MouseWheelUp
		}

		for range n {
			v.backend.Send(input.EncodeMouse(b, input.MousePress, x, max(y, 0), 0, v.modes))
		}

		return
	}

	k := input.KeyDown

	if up {
		k = input.KeyUp
	}

	for range n * altScrollLines {
		v.backend.Send(input.EncodeKey(k, 0, v.modes))
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}

func (v *View) paste(e transfer.DataEvent) {
	r := e.Open()
	defer r.Close()

	data, err := io.ReadAll(r)

	if err != nil || len(data) == 0 {
		return
	}

	v.word = v.word[:0]
	v.send(input.EncodePaste(string(data), v.modes))
}

func (v *View) send(p []byte) {
	if len(p) == 0 {
		return
	}

	v.offset = 0
	v.sel.clear()
	v.touchInput()
	v.backend.Send(p)
}

func (v *View) copySelection(gtx layout.Context) {
	if !v.sel.active {
		return
	}

	a, b := v.sel.bounds()
	text := v.backend.Terminal().TextRange(a, b)

	if text == "" {
		return
	}

	gtx.Execute(clipboard.WriteCmd{Type: mimeText, Data: io.NopCloser(strings.NewReader(text))})
}

func (v *View) selectAll() {
	f := &v.frame
	first := f.TopLine + f.Offset - f.Scrollback
	last := f.TopLine + f.Offset + f.Rows - 1
	v.sel = selection{active: true, anchor: vt.Pos{Line: first}, head: vt.Pos{Line: last, Col: f.Cols - 1}}
}

func (v *View) openMenu(at image.Point) {
	items := []kit.MenuItem{
		menuCopy:       {Label: "Copy", Icon: icons.Copy, Shortcut: "Ctrl+Shift+C", Disabled: !v.sel.active},
		menuPaste:      {Label: "Paste", Icon: icons.Paste, Shortcut: "Ctrl+Shift+V"},
		menuSelectAll:  {Label: "Select all", Shortcut: "Ctrl+Shift+A"},
		menuSep1:       kit.Separator(),
		menuFind:       {Label: "Search…", Icon: icons.Search, Shortcut: "Ctrl+Shift+F"},
		menuClear:      {Label: "Clear buffer"},
		menuSep2:       kit.Separator(),
		menuSplitRight: {Label: "Split right", Icon: icons.SplitRight, Shortcut: "Alt+Shift+="},
		menuSplitDown:  {Label: "Split down", Icon: icons.SplitDown, Shortcut: "Alt+Shift+-"},
		menuSep3:       kit.Separator(),
		menuClose:      {Label: "Close panel", Icon: icons.Close, Shortcut: "Ctrl+Shift+W"},
	}

	v.extras = nil

	if v.Extras != nil {
		v.extras = v.Extras()
	}

	if len(v.extras) > 0 {
		items = append(items, kit.Separator())

		for _, e := range v.extras {
			items = append(items, e.Item)
		}
	}

	v.menu.Open(items, at)
}

func (v *View) handleMenu(gtx layout.Context) {
	i, ok := v.menu.Chosen()

	if !ok {
		return
	}

	v.wantFocus = true

	switch i {
	case menuCopy:
		v.copySelection(gtx)
	case menuPaste:
		gtx.Execute(clipboard.ReadCmd{Tag: v})
	case menuSelectAll:
		v.selectAll()
	case menuFind:
		v.search.show()
	case menuClear:
		v.backend.Terminal().Clear()
		v.backend.Send([]byte{0x0c})
	case menuSplitRight:
		v.action(ActionSplitRight)
	case menuSplitDown:
		v.action(ActionSplitDown)
	case menuClose:
		v.action(ActionClosePane)
	default:
		if j := i - menuExtras - 1; j >= 0 && j < len(v.extras) {
			v.action(v.extras[j].Action)
		}
	}
}

func (v *View) action(a string) {
	if v.OnAction != nil {
		v.OnAction(a)
	}
}
