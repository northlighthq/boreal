package kit

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/northlighthq/boreal/internal/ui/icons"
)

func (k *Kit) EmptyState(gtx C, ic *icons.Icon, title, hint string, actions ...layout.Widget) D {
	th := k.Th

	return layout.Inset{Top: 56, Left: 24, Right: 24}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
		return layout.N.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))

			children := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					side := gtx.Dp(44)
					r := image.Rectangle{Max: image.Pt(side, side)}
					RRect(gtx, r, gtx.Dp(th.Radius), th.Hover)
					Border(gtx, r, gtx.Dp(th.Radius), 1, th.Border)
					gtx.Constraints = layout.Exact(r.Max)
					return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 22, th.TextMuted) })
				}),
				layout.Rigid(Gap(14)),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, TextStyle{Weight: font.SemiBold}, title) }),
			}

			if hint != "" {
				children = append(children,
					layout.Rigid(Gap(6)),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, TextStyle{Color: th.TextMuted, Size: th.TextSizeSmall, MaxLines: 4, Align: text.Middle}, hint)
					}),
				)
			}

			if len(actions) > 0 {
				row := make([]layout.FlexChild, 0, 2*len(actions))

				for i, act := range actions {
					if i > 0 {
						row = append(row, layout.Rigid(Gap(8)))
					}

					row = append(row, layout.Rigid(act))
				}

				children = append(children,
					layout.Rigid(Gap(18)),
					layout.Rigid(func(gtx C) D {
						wide := gtx
						wide.Constraints = layout.Constraints{Max: image.Pt(1<<20, gtx.Constraints.Max.Y)}
						rec := op.Record(gtx.Ops)
						dims := layout.Flex{Alignment: layout.Middle}.Layout(wide, row...)
						call := rec.Stop()

						if dims.Size.X <= gtx.Constraints.Max.X {
							call.Add(gtx.Ops)
							return dims
						}

						stack := make([]layout.FlexChild, 0, 2*len(actions))

						for i, act := range actions {
							if i > 0 {
								stack = append(stack, layout.Rigid(Gap(8)))
							}

							stack = append(stack, layout.Rigid(act))
						}

						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, stack...)
					}),
				)
			}

			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}
