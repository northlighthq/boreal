package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
)

type RowOpts struct {
	Lead     W
	Title    string
	Subtitle string
	Mono     bool
	Trailing W
	Chevron  bool
	Danger   bool
	Value    string
}

func Section(gtx C, k *kit.Kit, caption string, rows ...W) D {
	th := k.Th
	children := []layout.FlexChild{}

	if caption != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 4, Bottom: 8}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: TextCaption, Color: th.TextMuted, Weight: font.SemiBold, MaxLines: 1}, caption)
			})
		}))
	}

	children = append(children, layout.Rigid(func(gtx C) D {
		return Card(gtx, k, func(gtx C) D {
			items := make([]layout.FlexChild, 0, 2*len(rows))

			for i, r := range rows {
				if i > 0 {
					items = append(items, layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: Gutter}.Layout(gtx, func(gtx C) D {
							h := max(gtx.Dp(1)/2, 1)
							kit.Rect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, h), th.Divider)
							return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
						})
					}))
				}

				items = append(items, layout.Rigid(r))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
	}))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func Card(gtx C, k *kit.Kit, w W) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	dims := w(gtx)
	call := rec.Stop()
	r := image.Rectangle{Max: dims.Size}
	radius := gtx.Dp(CardRadius)
	kit.RRect(gtx, r, radius, k.Th.Panel)
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

func Row(gtx C, k *kit.Kit, clk *widget.Clickable, o RowOpts) D {
	if clk == nil {
		return rowBody(gtx, k, nil, o)
	}

	return clk.Layout(gtx, func(gtx C) D { return rowBody(gtx, k, clk, o) })
}

func rowBody(gtx C, k *kit.Kit, clk *widget.Clickable, o RowOpts) D {
	th := k.Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	minH := gtx.Dp(RowHeight)

	if o.Subtitle == "" {
		minH = gtx.Dp(MinTouch + 4)
	}

	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Left: Gutter, Right: 12}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, minH)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if o.Lead == nil {
					return D{}
				}

				return layout.Inset{Right: 14}.Layout(gtx, o.Lead)
			}),
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.Y = 0
				return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							fg := th.Text

							if o.Danger {
								fg = th.Danger
							}

							return k.Label(gtx, kit.TextStyle{Size: TextBody, Color: fg, MaxLines: 1}, o.Title)
						}),
						layout.Rigid(func(gtx C) D {
							if o.Subtitle == "" {
								return D{}
							}

							return layout.Inset{Top: 2}.Layout(gtx, func(gtx C) D {
								return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, MaxLines: 1, Mono: o.Mono}, o.Subtitle)
							})
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if o.Value == "" {
					return D{}
				}

				return layout.Inset{Left: 8}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, MaxLines: 1}, o.Value)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if o.Trailing == nil {
					return D{}
				}

				return layout.Inset{Left: 8}.Layout(gtx, o.Trailing)
			}),
			layout.Rigid(func(gtx C) D {
				if !o.Chevron {
					return D{}
				}

				return layout.Inset{Left: 6}.Layout(gtx, func(gtx C) D {
					return icons.ChevronRight.Layout(gtx, 18, th.TextFaint)
				})
			}),
		)
	})
	call := rec.Stop()

	if clk != nil && clk.Pressed() {
		kit.Rect(gtx, image.Rectangle{Max: dims.Size}, th.Pressed)
	}

	call.Add(gtx.Ops)
	return dims
}

func Tile(gtx C, ic *icons.Icon, fg, bg color.NRGBA) D {
	side := gtx.Dp(TileSize)
	r := image.Rectangle{Max: image.Pt(side, side)}
	kit.RRect(gtx, r, gtx.Dp(TileRadius), bg)
	gtx.Constraints = layout.Exact(r.Max)
	return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 21, fg) })
}

func TileFor(k *kit.Kit, ic *icons.Icon, tint color.NRGBA) W {
	return func(gtx C) D {
		th := k.Th
		fg := th.TextMuted
		bg := th.Hover

		if tint.A > 0 {
			fg = tint
			bg = kit.Fade(tint, 0.16)
		}

		return Tile(gtx, ic, fg, bg)
	}
}

func Caption(gtx C, k *kit.Kit, s string) D {
	return layout.Inset{Left: 4, Top: 8}.Layout(gtx, func(gtx C) D {
		return k.Label(gtx, kit.TextStyle{Size: TextCaption, Color: k.Th.TextMuted, MaxLines: 4}, s)
	})
}

func Page(gtx C, list *widget.List, children ...W) D {
	list.Axis = layout.Vertical
	return list.Layout(gtx, len(children), func(gtx C, i int) D {
		in := layout.Inset{Left: Gutter, Right: Gutter, Bottom: 22}

		if i == 0 {
			in.Top = 6
		}

		if i == len(children)-1 {
			in.Bottom = 96
		}

		return in.Layout(gtx, children[i])
	})
}
