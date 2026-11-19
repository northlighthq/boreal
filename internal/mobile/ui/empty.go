package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/mark"
)

func Empty(gtx C, k *kit.Kit, ic *icons.Icon, title, hint string, actions ...W) D {
	th := k.Th
	gtx.Constraints.Min = gtx.Constraints.Max

	return layout.Inset{Left: 32, Right: 32, Bottom: 72}.Layout(gtx, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(EmptyMaxWidth))
			children := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					if ic == nil {
						return mark.Paint(gtx, 34, kit.Fade(th.TextFaint, 0.55))
					}

					return ic.Layout(gtx, 30, th.TextFaint)
				}),
				layout.Rigid(Gap(16)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextBar, Weight: font.SemiBold, Align: text.Middle, MaxLines: 2}, title)
				}),
				layout.Rigid(Gap(6)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, Align: text.Middle, MaxLines: 5}, hint)
				}),
			}

			for i, a := range actions {
				gap := Gap(8)

				if i == 0 {
					gap = Gap(24)
				}

				children = append(children, layout.Rigid(gap), layout.Rigid(func(gtx C) D {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(EmptyMaxWidth-80))
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return a(gtx)
				}))
			}

			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}
