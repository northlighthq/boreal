package settings

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/kit"
)

const (
	rowPadX      = unit.Dp(16)
	rowPadY      = unit.Dp(12)
	rowMinHeight = unit.Dp(52)
	controlGap   = unit.Dp(24)
	groupGap     = unit.Dp(22)
	stackBelow   = unit.Dp(440)
	selectWidth  = unit.Dp(220)
	fieldWidth   = unit.Dp(100)
	wideField    = unit.Dp(260)
	dividerAlpha = 0.5
)

type page struct {
	k *kit.Kit
}

func (p page) header(title string) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return layout.Inset{Bottom: 16}.Layout(gtx, func(gtx C) D {
			return p.k.Label(gtx, kit.TextStyle{Size: p.k.Th.TextSizeLarge, Weight: font.SemiBold}, title)
		})
	})
}

func (p page) group(title string, rows ...layout.Widget) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return layout.Inset{Bottom: groupGap}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if title == "" {
						return D{}
					}

					return layout.Inset{Left: 2, Bottom: 8}.Layout(gtx, func(gtx C) D {
						return p.k.Label(gtx, kit.TextStyle{Size: p.k.Th.TextSizeSmall, Color: p.k.Th.TextMuted, Weight: font.Medium}, title)
					})
				}),
				layout.Rigid(func(gtx C) D { return p.card(gtx, rows) }),
			)
		})
	})
}

func (p page) card(gtx C, rows []layout.Widget) D {
	th := p.k.Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	items := make([]layout.FlexChild, 0, 2*len(rows))

	for i, r := range rows {
		if i > 0 {
			items = append(items, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: rowPadX}.Layout(gtx, func(gtx C) D {
					size := image.Pt(gtx.Constraints.Max.X, max(gtx.Dp(1)/2, 1))
					kit.Rect(gtx, image.Rectangle{Max: size}, kit.Fade(th.Divider, dividerAlpha))
					return D{Size: size}
				})
			}))
		}

		items = append(items, layout.Rigid(r))
	}

	rec := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
	call := rec.Stop()
	r := image.Rectangle{Max: dims.Size}
	radius := gtx.Dp(th.Radius)
	kit.RRect(gtx, r, radius, th.Panel)
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

func (p page) item(title, desc string, control layout.Widget) layout.Widget {
	return p.itemColor(title, desc, p.k.Th.TextMuted, control)
}

func (p page) itemColor(title, desc string, descColor color.NRGBA, control layout.Widget) layout.Widget {
	k := p.k
	th := k.Th

	text := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{MaxLines: 2}, title) }),
			layout.Rigid(func(gtx C) D {
				if desc == "" {
					return D{}
				}

				return layout.Inset{Top: 3}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: descColor, MaxLines: 4}, desc)
				})
			}),
		)
	}

	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		stacked := gtx.Constraints.Max.X < gtx.Dp(stackBelow)

		return layout.Inset{Left: rowPadX, Right: rowPadX, Top: rowPadY, Bottom: rowPadY}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(rowMinHeight - 2*rowPadY)

			if control == nil {
				return layout.W.Layout(gtx, text)
			}

			if stacked {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(text),
					layout.Rigid(kit.Gap(10)),
					layout.Rigid(control),
				)
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, text),
				layout.Rigid(kit.Gap(controlGap)),
				layout.Rigid(control),
			)
		})
	}
}

func (p page) block(w layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Left: rowPadX, Right: rowPadX, Top: rowPadY, Bottom: rowPadY}.Layout(gtx, w)
	}
}

func (p page) note(text string, c color.NRGBA) layout.Widget {
	return p.block(func(gtx C) D {
		return p.k.Label(gtx, kit.TextStyle{Size: p.k.Th.TextSizeSmall, Color: c, MaxLines: 6}, text)
	})
}

func (p page) toggle(b *widget.Bool) layout.Widget {
	return func(gtx C) D { return p.k.Toggle(gtx, b) }
}

func (p page) width(w unit.Dp, inner layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(w))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return inner(gtx)
	}
}

func (p page) field(ed *widget.Editor, hint string, w unit.Dp) layout.Widget {
	return p.width(w, func(gtx C) D { return p.k.Field(gtx, ed, kit.FieldOpts{Hint: hint}) })
}

func (p page) choose(s *kit.Select, options []string) layout.Widget {
	return p.width(selectWidth, func(gtx C) D { return p.k.Select(gtx, s, options) })
}

func (p page) segmented(s *kit.Segmented, options []string) layout.Widget {
	return func(gtx C) D { return p.k.Segmented(gtx, s, options) }
}
