package terminal

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/northlighthq/vt/input"

	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const (
	barHeight = unit.Dp(48)
	chipH     = unit.Dp(36)
	chipMinW  = unit.Dp(44)
)

type barKey struct {
	label string
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
	{label: "←", key: input.KeyLeft},
	{label: "↑", key: input.KeyUp},
	{label: "↓", key: input.KeyDown},
	{label: "→", key: input.KeyRight},
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

	return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx C) D {
		return b.list.Layout(gtx, len(barKeys), func(gtx C, i int) D {
			return layout.Inset{Left: 3, Right: 3}.Layout(gtx, func(gtx C) D {
				return layout.W.Layout(gtx, func(gtx C) D { return b.chip(gtx, k, s, i) })
			})
		})
	})
}

func (b *keyBar) chip(gtx C, k *kit.Kit, s *Session, i int) D {
	th := k.Th
	bk := barKeys[i]
	on := (bk.latch == latchCtrl && s.ctrl) || (bk.latch == latchAlt && s.alt)
	clk := &b.clks[i]

	return clk.Layout(gtx, func(gtx C) D {
		bg, fg := kit.Lerp(th.Panel, th.Text, 0.09), th.Text

		if on {
			bg, fg = th.Accent, th.OnAccent
		} else if clk.Pressed() {
			bg = th.Pressed
		}

		gtx.Constraints.Min = image.Pt(gtx.Dp(chipMinW), gtx.Dp(chipH))
		gtx.Constraints.Max.Y = gtx.Dp(chipH)
		rec := op.Record(gtx.Ops)
		dims := layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: ui.TextSub, Color: fg, Weight: font.Medium, MaxLines: 1}, bk.label)
			})
		})
		call := rec.Stop()
		dims.Size.X = max(dims.Size.X, gtx.Dp(chipMinW))
		kit.RRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(9), bg)
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
