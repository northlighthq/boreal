package sol

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mascot"
)

type Sol struct {
	Backdrop color.NRGBA
	click    gesture.Click
	start    time.Time
	last     time.Time
	t        float64
	happy    time.Time
	origin   f32.Point
	known    bool
	Follow   bool
	Still    bool
	Posed    bool
	Tilt     float32
	Look     f32.Point

	body    op.Ops
	bodyOp  op.CallOp
	bodyKey solBodyKey
}

type solBodyKey struct {
	px     int
	t      float64
	colors mascot.Colors
	ok     bool
}

const solHappy = 1200 * time.Millisecond

func (s *Sol) Layout(gtx layout.Context, k *kit.Kit, size unit.Dp) layout.Dimensions {
	px := gtx.Dp(size)
	now := gtx.Now

	if s.start.IsZero() {
		s.start, s.last = now, now
	}

	hovered := s.click.Hovered()

	for {
		e, ok := s.click.Update(gtx.Source)

		if !ok {
			break
		}

		if e.Kind == gesture.KindClick {
			s.happy = now
		}
	}

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: s, Kinds: pointer.Move | pointer.Enter | pointer.Drag})

		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok {
			s.origin = k.Pointer().Sub(e.Position)
			s.known = true
		}
	}

	if hovered {
		s.t += now.Sub(s.last).Seconds()
	}

	s.last = now
	wall := now.Sub(s.start).Seconds()

	st := mascot.State{T: s.t}

	if !s.Still {
		st.Blink = mascot.BlinkAt(wall)
	}

	if d := now.Sub(s.happy); !s.happy.IsZero() && d < solHappy {
		st.Happy = float32(math.Min(1, 1.6*(1-d.Seconds()/solHappy.Seconds())))
	}

	if s.Posed {
		st.Look = s.Look
	} else if s.known {
		c := s.origin.Add(f32.Pt(float32(px)/2, float32(px)/2))
		d := k.Pointer().Sub(c)
		reach := float32(gtx.Dp(260))

		if !s.Follow {
			reach = float32(gtx.Dp(60))
		}

		st.Look = f32.Pt(d.X/reach, d.Y/reach)
	}

	var jump float32

	if st.Happy > 0 {
		ph := now.Sub(s.happy).Seconds()
		jump = float32(math.Abs(math.Sin(ph*9))*math.Exp(-ph*3.5)) * float32(px) * 0.08
	}

	center := f32.Pt(float32(px)/2, float32(px)/2)
	tr := f32.Affine2D{}.Rotate(center, s.Tilt).Offset(f32.Pt(0, -jump))
	off := op.Affine(tr).Push(gtx.Ops)
	colors := mascot.DefaultColors

	if !k.Th.Dark {
		colors = mascot.LightColors
	}

	colors.Backdrop = s.Backdrop

	if key := (solBodyKey{px: px, t: st.T, colors: colors, ok: true}); key != s.bodyKey {
		s.body.Reset()
		m := op.Record(&s.body)
		paint.NewImageOp(mascot.RenderBody(px, st, colors)).Add(&s.body)
		paint.PaintOp{}.Add(&s.body)
		s.bodyOp, s.bodyKey = m.Stop(), key
	}

	s.bodyOp.Add(gtx.Ops)
	mascot.DrawEyes(gtx.Ops, float32(px), st, colors)
	off.Pop()

	area := clip.Rect{Max: image.Pt(px, px)}.Push(gtx.Ops)
	s.click.Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()

	if s.Follow {
		pass := pointer.PassOp{}.Push(gtx.Ops)
		big := clip.Rect{Min: image.Pt(-1<<14, -1<<14), Max: image.Pt(1<<14, 1<<14)}.Push(gtx.Ops)
		event.Op(gtx.Ops, s)
		big.Pop()
		pass.Pop()
	} else {
		area := clip.Rect{Max: image.Pt(px, px)}.Push(gtx.Ops)
		event.Op(gtx.Ops, s)
		area.Pop()
	}

	switch {
	case hovered || st.Happy > 0 || st.Blink > 0:
		gtx.Execute(op.InvalidateCmd{})
	case s.Still:
	default:
		p := math.Mod(wall, mascot.Period)
		next := mascot.Period - 0.5 - p

		if next <= 0 {
			gtx.Execute(op.InvalidateCmd{At: now.Add(30 * time.Millisecond)})
		} else {
			gtx.Execute(op.InvalidateCmd{At: now.Add(time.Duration(next * float64(time.Second)))})
		}
	}

	return layout.Dimensions{Size: image.Pt(px, px)}
}

func (s *Sol) Cheer(now time.Time) {
	s.happy = now
}

func (s *Sol) Cheering(now time.Time) bool {
	return !s.happy.IsZero() && now.Sub(s.happy) < solHappy
}
