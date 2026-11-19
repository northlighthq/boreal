package terminal

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/northlighthq/vt/input"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const (
	barHeight = unit.Dp(48)
	chipH     = unit.Dp(34)
	chipMinW  = unit.Dp(42)
	chipPadX  = unit.Dp(11)
	chipGap   = unit.Dp(6)
	fadeWidth = unit.Dp(32)
	arrowSize = unit.Dp(17)
)

type barKey struct {
	label string
	icon  *icons.Icon
	key   input.Key
	text  string
	latch int
}

const (
	latchNone = iota
	latchCtrl
	latchAlt
)

var barKeys = [...]barKey{
	{label: "Esc", key: input.KeyEscape},
	{label: "Tab", key: input.KeyTab},
	{label: "Ctrl", latch: latchCtrl},
	{label: "Alt", latch: latchAlt},
	{icon: icons.ArrowLeft, key: input.KeyLeft},
	{icon: icons.ArrowUp, key: input.KeyUp},
	{icon: icons.ArrowDown, key: input.KeyDown},
	{icon: icons.ArrowRight, key: input.KeyRight},
	{label: "|", text: "|"},
	{label: "/", text: "/"},
	{label: "-", text: "-"},
	{label: "~", text: "~"},
	{label: "Home", key: input.KeyHome},
	{label: "End", key: input.KeyEnd},
	{label: "PgUp", key: input.KeyPageUp},
	{label: "PgDn", key: input.KeyPageDown},
}

type keyBar struct {
	clks [len(barKeys)]widget.Clickable
	list widget.List
}

func (b *keyBar) Layout(gtx C, k *kit.Kit, s *Session) D {
	th := k.Th

	for i := range b.clks {
		if b.clks[i].Clicked(gtx) {
			b.press(s, barKeys[i])
		}
	}

	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(barHeight))
	kit.Rect(gtx, image.Rectangle{Max: size}, th.Panel)
	kit.Rect(gtx, image.Rect(0, 0, size.X, max(gtx.Dp(1)/2, 1)), th.Divider)
	gtx.Constraints = layout.Exact(size)
	b.list.Axis = layout.Horizontal

	layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
		return b.list.Layout(gtx, len(barKeys), func(gtx C, i int) D {
			in := layout.Inset{Right: chipGap}

			if i == len(barKeys)-1 {
				in.Right = 0
			}

			return in.Layout(gtx, func(gtx C) D {
				return layout.W.Layout(gtx, func(gtx C) D { return b.chip(gtx, k, s, i) })
			})
		})
	})

	if b.list.Position.BeforeEnd {
		fade(gtx, size, gtx.Dp(fadeWidth), th.Panel)
	}

	return D{Size: size}
}

func fade(gtx C, size image.Point, width int, c color.NRGBA) {
	x0, x1 := float32(size.X-width), float32(size.X)
	clear := c
	clear.A = 0
	defer clip.Rect{Min: image.Pt(size.X-width, 1), Max: size}.Push(gtx.Ops).Pop()
	paint.LinearGradientOp{Stop1: f32.Pt(x0, 0), Color1: clear, Stop2: f32.Pt(x1, 0), Color2: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (b *keyBar) chip(gtx C, k *kit.Kit, s *Session, i int) D {
	th := k.Th
	bk := barKeys[i]
	on := (bk.latch == latchCtrl && s.ctrl) || (bk.latch == latchAlt && s.alt)
	clk := &b.clks[i]

	return clk.Layout(gtx, func(gtx C) D {
		bg, fg := th.Selection, th.Text

		switch {
		case on:
			bg, fg = th.Accent, th.OnAccent
		case clk.Pressed():
			bg = kit.Lerp(th.Selection, th.Text, 0.12)
		}

		h := gtx.Dp(chipH)
		gtx.Constraints.Min = image.Pt(gtx.Dp(chipMinW), h)
		gtx.Constraints.Max.Y = h
		rec := op.Record(gtx.Ops)
		dims := layout.Inset{Left: chipPadX, Right: chipPadX}.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				if bk.icon != nil {
					return bk.icon.Layout(gtx, arrowSize, fg)
				}

				return k.Label(gtx, kit.TextStyle{Size: ui.TextSub, Color: fg, Mono: true, MaxLines: 1}, bk.label)
			})
		})
		call := rec.Stop()
		dims.Size.X = max(dims.Size.X, gtx.Dp(chipMinW))
		r := image.Rectangle{Max: dims.Size}
		radius := gtx.Dp(th.RadiusSmall)
		kit.RRect(gtx, r, radius, bg)
		call.Add(gtx.Ops)
		return dims
	})
}

func (b *keyBar) press(s *Session, bk barKey) {
	switch bk.latch {
	case latchCtrl:
		s.ctrl = !s.ctrl
		return
	case latchAlt:
		s.alt = !s.alt
		return
	}

	if bk.text != "" {
		s.View.SendText(s.transform(bk.text))
		return
	}

	s.View.SendKey(bk.key, s.mods())
}
