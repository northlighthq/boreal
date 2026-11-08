package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
)

const emptyTile = unit.Dp(72)

func Empty(gtx C, k *kit.Kit, ic *icons.Icon, title, hint string, actions ...W) D {
	th := k.Th
	gtx.Constraints.Min = gtx.Constraints.Max

	return layout.Inset{Left: 32, Right: 32, Bottom: 48}.Layout(gtx, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(340))
			children := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					side := gtx.Dp(emptyTile)
					r := image.Rectangle{Max: image.Pt(side, side)}
					kit.RRect(gtx, r, gtx.Dp(22), kit.Fade(th.Accent, 0.12))
					gtx.Constraints = layout.Exact(r.Max)
					return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 32, th.Accent) })
				}),
				layout.Rigid(Gap(20)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextBar + 1, Weight: font.SemiBold, Align: text.Middle, MaxLines: 2}, title)
				}),
				layout.Rigid(Gap(8)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, Align: text.Middle, MaxLines: 5}, hint)
				}),
			}

			for i, a := range actions {
				gap := unit.Dp(10)

				if i == 0 {
					gap = 28
				}

				children = append(children, layout.Rigid(Gap(gap)), layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return a(gtx)
				}))
			}

			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}
