package kit

import (
	"image"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/plattyssh/platty/internal/ui/theme"
)

const doubleClick = 400 * time.Millisecond

type Row struct {
	clk       widget.Clickable
	lastClick time.Time
	secondary bool
	menuAt    image.Point
}

type RowEvents struct {
	Click       bool
	DoubleClick bool
	Context     bool
	At          image.Point
}

func (r *Row) Update(gtx C) RowEvents {
	var ev RowEvents

	for {
		c, ok := r.clk.Update(gtx)

		if !ok {
			break
		}

		ev.Click = true

		if c.NumClicks >= 2 || time.Since(r.lastClick) < doubleClick {
			ev.DoubleClick = true
			r.lastClick = time.Time{}
			continue
		}

		r.lastClick = time.Now()
	}

	for {
		e, ok := gtx.Event(pointer.Filter{Target: &r.secondary, Kinds: pointer.Press})

		if !ok {
			break
		}

		if pe, ok := e.(pointer.Event); ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
			ev.Context = true
			ev.At = pe.Position.Round()
		}
	}

	if ev.Click || ev.Context {
		gtx.Execute(op.InvalidateCmd{})
	}

	return ev
}

func (r *Row) Hovered() bool {
	return r.clk.Hovered()
}

type RowOpts struct {
	Plain    bool
	Selected bool
	Focused  bool
	Height   unit.Dp
	Indent   unit.Dp
}

func (k *Kit) Row(gtx C, r *Row, o RowOpts, w W) D {
	th := k.Th
	h := o.Height

	if h == 0 {
		h = th.RowHeight
	}

	return r.clk.Layout(gtx, func(gtx C) D {
		height := gtx.Dp(h)
		width := gtx.Constraints.Max.X
		size := image.Pt(width, height)

		hv := k.Hover(gtx, Key(&r.clk, keyHover), r.clk.Hovered())
		sel := k.Animated(gtx, Key(r, keyActive), B2F(o.Selected), Normal)
		selColor := th.Inactive

		if o.Focused {
			selColor = th.Selection
		}

		inner := image.Rect(gtx.Dp(6), 1, width-gtx.Dp(6), height-1)
		radius := gtx.Dp(th.RadiusSmall)

		if bg := Lerp(Fade(th.Hover, hv), selColor, sel); bg.A > 0 && !o.Plain {
			RRect(gtx, inner, radius, bg)
		}

		area := clip.Rect{Max: size}.Push(gtx.Ops)
		event.Op(gtx.Ops, &r.secondary)
		pointer.CursorPointer.Add(gtx.Ops)
		area.Pop()

		gtx.Constraints = layout.Exact(size)
		layout.Inset{Left: 12 + o.Indent, Right: 12}.Layout(gtx, func(gtx C) D {
			cg := gtx
			cg.Constraints.Min.Y = 0
			rec := op.Record(gtx.Ops)
			d := w(cg)
			call := rec.Stop()
			off := op.Offset(image.Pt(0, (height-d.Size.Y)/2)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			off.Pop()
			return D{Size: image.Pt(d.Size.X, height)}
		})
		return D{Size: size}
	})
}

type ScrollList struct {
	widget.List
	last   layout.Position
	moved  time.Time
	inside bool
	fade   Anim
}

const scrollLinger = 900 * time.Millisecond

func (k *Kit) List(gtx C, l *ScrollList, n int, el layout.ListElement) D {
	l.Axis = layout.Vertical

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: l, Kinds: pointer.Enter | pointer.Leave | pointer.Move})

		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok {
			l.inside = e.Kind != pointer.Leave
		}
	}

	if p := l.Position; p.First != l.last.First || p.Offset != l.last.Offset {
		l.last, l.moved = p, gtx.Now
	}

	recent := gtx.Now.Sub(l.moved) < scrollLinger

	if recent {
		gtx.Execute(op.InvalidateCmd{At: l.moved.Add(scrollLinger)})
	}

	a := l.fade.Animate(gtx, B2F(l.inside || recent), Normal)

	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, l)

	st := material.List(k.Mat, &l.List)
	st.Indicator.Color = theme.WithAlpha(k.Th.TextFaint, uint8(0x60*a))
	st.Indicator.HoverColor = theme.WithAlpha(k.Th.TextMuted, uint8(0xa0*a))
	st.Indicator.MinorWidth = 4
	st.Indicator.CornerRadius = 2
	st.Track.MinorPadding = 2
	st.AnchorStrategy = material.Overlay
	dims := st.Layout(gtx, n, el)
	area.Pop()
	return dims
}
