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
	Dot      color.NRGBA
}

func Section(gtx C, k *kit.Kit, caption string, rows ...W) D {
	return SectionMeta(gtx, k, caption, "", rows...)
}

func SectionMeta(gtx C, k *kit.Kit, caption, meta string, rows ...W) D {
	children := []layout.FlexChild{}

	if caption != "" || meta != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 4, Right: 4, Bottom: 8}.Layout(gtx, func(gtx C) D { return heading(gtx, k, caption, meta) })
		}))
	}

	children = append(children, layout.Rigid(func(gtx C) D {
		return Card(gtx, k, func(gtx C) D {
			items := make([]layout.FlexChild, 0, 2*len(rows))

			for i, r := range rows {
				if i > 0 {
					items = append(items, layout.Rigid(func(gtx C) D { return Divider(gtx, k) }))
				}

				items = append(items, layout.Rigid(r))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
	}))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func Group(gtx C, k *kit.Kit, caption string, items ...W) D {
	children := make([]layout.FlexChild, 0, 2*len(items)+1)

	if caption != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 4, Bottom: 10}.Layout(gtx, func(gtx C) D { return heading(gtx, k, caption, "") })
		}))
	}

	for i, it := range items {
		if i > 0 {
			children = append(children, layout.Rigid(Gap(14)))
		}

		children = append(children, layout.Rigid(it))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func heading(gtx C, k *kit.Kit, caption, meta string) D {
	th := k.Th
	style := kit.TextStyle{Size: TextSub, Color: th.TextMuted, Weight: font.Medium, MaxLines: 1}

	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return k.Label(gtx, style, caption) }),
		layout.Rigid(func(gtx C) D {
			if meta == "" {
				return D{}
			}

			style.Color, style.Weight = th.TextFaint, font.Normal
			return k.Label(gtx, style, meta)
		}),
	)
}

func Divider(gtx C, k *kit.Kit) D {
	return layout.Inset{Left: Gutter}.Layout(gtx, func(gtx C) D {
		h := max(gtx.Dp(1)/2, 1)
		kit.Rect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, h), k.Th.Divider)
		return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
	})
}

func Card(gtx C, k *kit.Kit, w W) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	dims := w(gtx)
	call := rec.Stop()
	r := image.Rectangle{Max: dims.Size}
	radius := gtx.Dp(k.Th.RadiusLarge)
	kit.RRect(gtx, r, radius, k.Th.Popup)
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

func Row(gtx C, k *kit.Kit, clk *widget.Clickable, o RowOpts) D {
	if clk == nil {
		return rowBody(gtx, k, nil, o)
	}

	if o.Trailing == nil {
		return clk.Layout(gtx, func(gtx C) D { return rowBody(gtx, k, clk, o) })
	}

	g := gtx
	g.Constraints.Min = image.Point{}
	rec := op.Record(gtx.Ops)
	tdims := o.Trailing(g)
	trailing := rec.Stop()

	o.Trailing = func(gtx C) D { return D{Size: tdims.Size} }
	dims := clk.Layout(gtx, func(gtx C) D { return rowBody(gtx, k, clk, o) })

	off := image.Pt(dims.Size.X-gtx.Dp(8)-tdims.Size.X, (dims.Size.Y-tdims.Size.Y)/2)
	if o.Chevron {
		off.X -= gtx.Dp(24)
	}

	stack := op.Offset(off).Push(gtx.Ops)
	trailing.Add(gtx.Ops)
	stack.Pop()
	return dims
}

func rowBody(gtx C, k *kit.Kit, clk *widget.Clickable, o RowOpts) D {
	th := k.Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	minH := gtx.Dp(RowHeight)

	if o.Subtitle == "" {
		minH = gtx.Dp(MinTouch + 4)
	}

	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Left: Gutter, Right: 8}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, minH)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if o.Lead == nil {
					return D{}
				}

				return layout.Inset{Right: 12}.Layout(gtx, o.Lead)
			}),
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.Y = 0
				return layout.Inset{Top: 10, Bottom: 10, Right: 8}.Layout(gtx, func(gtx C) D {
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
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										if o.Dot.A == 0 {
											return D{}
										}

										return layout.Inset{Right: 6}.Layout(gtx, func(gtx C) D { return kit.Dot(gtx, o.Dot, 7) })
									}),
									layout.Flexed(1, func(gtx C) D {
										return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, MaxLines: 1, Mono: o.Mono}, o.Subtitle)
									}),
								)
							})
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if o.Value == "" {
					return D{}
				}

				return layout.Inset{Right: 4}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextFaint, MaxLines: 1}, o.Value)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if o.Trailing == nil {
					return D{}
				}

				return o.Trailing(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if !o.Chevron {
					return D{}
				}

				return layout.Inset{Left: 2, Right: 6}.Layout(gtx, func(gtx C) D {
					return icons.ChevronRight.Layout(gtx, 16, th.TextFaint)
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

func Tile(gtx C, ic *icons.Icon, fg, _ color.NRGBA) D {
	size := image.Pt(gtx.Dp(LeadWidth), gtx.Dp(LeadWidth))
	gtx.Constraints = layout.Exact(size)
	return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, LeadSize, fg) })
}

func TileFor(k *kit.Kit, ic *icons.Icon, tint color.NRGBA) W {
	return func(gtx C) D {
		fg := k.Th.TextMuted

		if tint.A > 0 {
			fg = tint
		}

		return Tile(gtx, ic, fg, color.NRGBA{})
	}
}

func Caption(gtx C, k *kit.Kit, s string) D {
	return layout.Inset{Left: 4, Top: 6}.Layout(gtx, func(gtx C) D {
		return k.Label(gtx, kit.TextStyle{Size: TextCaption, Color: k.Th.TextMuted, MaxLines: 4}, s)
	})
}

func Page(gtx C, list *widget.List, children ...W) D {
	list.Axis = layout.Vertical
	return list.Layout(gtx, len(children), func(gtx C, i int) D {
		in := layout.Inset{Left: Gutter, Right: Gutter, Bottom: SectionGap}

		if i == 0 {
			in.Top = 4
		}

		if i == len(children)-1 {
			in.Bottom = 40
		}

		return in.Layout(gtx, children[i])
	})
}
