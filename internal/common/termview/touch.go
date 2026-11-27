package termview

import (
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/northlighthq/vt/input"
)

const (
	touchSlop     = unit.Dp(8)
	flingMin      = unit.Dp(300)
	flingStop     = unit.Dp(30)
	flingFriction = 3.5
	flingSmooth   = 0.6
	flingStale    = 80 * time.Millisecond
	flingMaxStep  = 50 * time.Millisecond
)

type touchState struct {
	active bool
	moved  bool
	start  f32.Point
	last   f32.Point
	at     time.Duration
	vel    float32
}

type flingState struct {
	vel  float32
	last time.Time
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
		v.fling = flingState{}
		v.touch = touchState{active: true, start: e.Position, last: e.Position, at: e.Time}
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

		if dt := (e.Time - v.touch.at).Seconds(); dt > 0 {
			v.touch.vel = flingSmooth*(dy/float32(dt)) + (1-flingSmooth)*v.touch.vel
		}

		v.touch.last, v.touch.at = e.Position, e.Time
		v.scroll(pointer.Event{Position: e.Position, Scroll: f32.Pt(0, -dy), Source: pointer.Touch})
	case pointer.Release, pointer.Cancel:
		if v.touch.active && !v.touch.moved && e.Kind == pointer.Release {
			v.ShowKeyboard()
		}

		fresh := e.Time-v.touch.at < flingStale
		vel := v.touch.vel

		if e.Kind == pointer.Release && v.touch.moved && fresh && abs32(vel) > float32(gtx.Dp(flingMin)) && v.scrollsBack() {
			v.fling = flingState{vel: vel, last: gtx.Now}
			gtx.Execute(op.InvalidateCmd{})
		}

		v.touch = touchState{}
	}

	return true
}

func (v *View) scrollsBack() bool {
	return !v.modes.AltScreen && !v.reporting(0)
}

func (v *View) applyFling(gtx layout.Context) {
	if v.fling.vel == 0 {
		return
	}

	if !v.scrollsBack() {
		v.fling = flingState{}
		return
	}

	dt := min(gtx.Now.Sub(v.fling.last), flingMaxStep)
	v.fling.last = gtx.Now
	secs := float32(dt.Seconds())
	before := v.offset
	v.scroll(pointer.Event{Scroll: f32.Pt(0, -v.fling.vel*secs), Source: pointer.Touch})
	v.fling.vel *= float32(math.Exp(-flingFriction * float64(secs)))
	stalled := before == v.offset && v.offset == 0 && v.fling.vel < 0

	if abs32(v.fling.vel) < float32(gtx.Dp(flingStop)) || stalled {
		v.fling = flingState{}
		return
	}

	gtx.Execute(op.InvalidateCmd{})
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}

	return f
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
