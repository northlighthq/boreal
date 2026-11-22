package terminal

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/northlighthq/vt/input"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const (
	barHeight  = unit.Dp(50)
	chipH      = unit.Dp(36)
	chipGap    = unit.Dp(5)
	barPadX    = unit.Dp(8)
	arrowSize  = unit.Dp(17)
	pageKeys   = 8
	pageCount  = 4
	moreLabel  = "···"
	shortLabel = 3
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

var barPages = [pageCount][pageKeys]barKey{
	{
		{label: "Esc", key: input.KeyEscape},
		{label: "Tab", key: input.KeyTab},
		{label: "Ctrl", latch: latchCtrl},
		{label: "Alt", latch: latchAlt},
		{icon: icons.ArrowLeft, key: input.KeyLeft},
		{icon: icons.ArrowUp, key: input.KeyUp},
		{icon: icons.ArrowDown, key: input.KeyDown},
		{icon: icons.ArrowRight, key: input.KeyRight},
	},
	{
		{label: "|", text: "|"},
		{label: "/", text: "/"},
		{label: "-", text: "-"},
		{label: "~", text: "~"},
		{label: "Home", key: input.KeyHome},
		{label: "End", key: input.KeyEnd},
		{label: "PgUp", key: input.KeyPageUp},
		{label: "PgDn", key: input.KeyPageDown},
	},
	{
		{label: "F1", key: input.KeyF1},
		{label: "F2", key: input.KeyF2},
		{label: "F3", key: input.KeyF3},
		{label: "F4", key: input.KeyF4},
		{label: "F5", key: input.KeyF5},
		{label: "F6", key: input.KeyF6},
		{label: "F7", key: input.KeyF7},
		{label: "F8", key: input.KeyF8},
	},
	{
		{label: "F9", key: input.KeyF9},
		{label: "F10", key: input.KeyF10},
		{label: "F11", key: input.KeyF11},
		{label: "F12", key: input.KeyF12},
		{label: "Ins", key: input.KeyInsert},
		{label: "Del", key: input.KeyDelete},
		{label: "^C", text: "\x03"},
		{label: "^D", text: "\x04"},
	},
}

type keyBar struct {
	clks [pageCount][pageKeys]widget.Clickable
	more widget.Clickable
	menu widget.Clickable
	page int
}

func (b *keyBar) Layout(gtx C, k *kit.Kit, s *Session, compact bool) D {
	th := k.Th

	for p := range b.clks {
		for i := range b.clks[p] {
			if b.clks[p][i].Clicked(gtx) {
				b.press(s, barPages[p][i])
			}
		}
	}

	if b.more.Clicked(gtx) {
		b.page = (b.page + 1) % pageCount
	}

	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(barHeight))
	kit.Rect(gtx, image.Rectangle{Max: size}, th.Panel)
	kit.Rect(gtx, image.Rect(0, 0, size.X, max(gtx.Dp(1)/2, 1)), th.Divider)
	gtx.Constraints = layout.Exact(size)

	children := make([]layout.FlexChild, 0, 2*pageKeys+3)

	for i := range pageKeys {
		bk := barPages[b.page][i]
		on := (bk.latch == latchCtrl && s.ctrl) || (bk.latch == latchAlt && s.alt)
		children = append(children,
			layout.Flexed(1, func(gtx C) D { return b.chip(gtx, k, &b.clks[b.page][i], bk, on) }),
			layout.Rigid(ui.Gap(chipGap)),
		)
	}

	children = append(children, layout.Flexed(1, func(gtx C) D {
		return b.chip(gtx, k, &b.more, barKey{label: moreLabel}, b.page > 0)
	}))

	if compact {
		children = append(children,
			layout.Rigid(ui.Gap(chipGap)),
			layout.Flexed(1, func(gtx C) D { return b.chip(gtx, k, &b.menu, barKey{icon: icons.LucideMore}, false) }),
		)
	}

	layout.Inset{Left: barPadX, Right: barPadX}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})

	return D{Size: size}
}

func (b *keyBar) chip(gtx C, k *kit.Kit, clk *widget.Clickable, bk barKey, on bool) D {
	th := k.Th

	return clk.Layout(gtx, func(gtx C) D {
		bg, fg := th.Selection, th.Text

		switch {
		case on:
			bg, fg = th.Accent, th.OnAccent
		case clk.Pressed():
			bg = kit.Lerp(th.Selection, th.Text, 0.14)
		}

		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(chipH))
		kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), bg)
		gtx.Constraints = layout.Exact(size)

		return layout.Center.Layout(gtx, func(gtx C) D {
			if bk.icon != nil {
				return bk.icon.Layout(gtx, arrowSize, fg)
			}

			ts := ui.TextSub

			if len(bk.label) > shortLabel {
				ts = ui.TextCaption - 1
			}

			return k.Label(gtx, kit.TextStyle{Size: ts, Color: fg, Mono: true, MaxLines: 1}, bk.label)
		})
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
