package termview

import (
	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/northlighthq/vt/input"
)

const touchSlop = unit.Dp(8)

type touchState struct {
	active bool
	moved  bool
	start  f32.Point
	last   f32.Point
}

func (v *View) SendKey(k input.Key, mods input.Mod) {
	v.send(input.EncodeKey(k, mods, v.modes))
}

func (v *View) ShowKeyboard() {
	v.wantFocus = true
	v.wantKeyboard = true
}

func (v *View) handleTouch(gtx layout.Context, e pointer.Event) bool {
	if !v.set.Touch || e.Source != pointer.Touch {
		return false
	}

	switch e.Kind {
	case pointer.Press:
		v.touch = touchState{active: true, start: e.Position, last: e.Position}
	case pointer.Drag:
		if !v.touch.active {
			return true
		}

		if !v.touch.moved {
			dy := e.Position.Y - v.touch.start.Y

			if dy < 0 {
				dy = -dy
			}

			if dy < float32(gtx.Dp(touchSlop)) {
				return true
			}

			v.touch.moved = true
		}

		dy := e.Position.Y - v.touch.last.Y
		v.touch.last = e.Position
		v.scroll(pointer.Event{Position: e.Position, Scroll: f32.Pt(0, -dy), Source: pointer.Touch})
	case pointer.Release, pointer.Cancel:
		if v.touch.active && !v.touch.moved && e.Kind == pointer.Release {
			v.ShowKeyboard()
		}

		v.touch = touchState{}
	}

	return true
}

func (v *View) inputHint() key.InputHint {
	if v.set.Touch {
		return key.HintPassword
	}

	return key.HintAny
}

func (v *View) applyKeyboard(gtx layout.Context) {
	if !v.wantKeyboard {
		return
	}

	v.wantKeyboard = false
	gtx.Execute(key.SoftKeyboardCmd{Show: true})
}
