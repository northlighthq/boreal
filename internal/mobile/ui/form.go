package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
)

const (
	FormRowHeight  = unit.Dp(52)
	FormLabelWidth = unit.Dp(104)
	FormAreaHeight = unit.Dp(120)
)

type FormOpts struct {
	Hint     string
	Mono     bool
	Invalid  bool
	Trailing W
}

func FormRow(gtx C, k *kit.Kit, label string, ed *widget.Editor, o FormOpts) D {
	th := k.Th
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(FormRowHeight))
	k.FieldArea(gtx, ed, size)
	gtx.Constraints = layout.Exact(size)
	fg := th.Text

	if o.Invalid {
		fg = th.Danger
	}

	return layout.Inset{Left: Gutter, Right: 6}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(FormLabelWidth)
				gtx.Constraints.Max.X = gtx.Constraints.Min.X
				return layout.W.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextBody, Color: fg, MaxLines: 1}, label)
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				return centerY(gtx, func(gtx C) D { return Editor(gtx, k, ed, o.Hint, o.Mono) })
			}),
			layout.Rigid(func(gtx C) D {
				if o.Trailing == nil {
					return layout.Spacer{Width: 10}.Layout(gtx)
				}

				return o.Trailing(gtx)
			}),
		)
	})
}

func FormArea(gtx C, k *kit.Kit, ed *widget.Editor, o FormOpts) D {
	minH := gtx.Dp(FormAreaHeight)
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, minH)

	if gtx.Constraints.Max.Y < minH {
		gtx.Constraints.Max.Y = minH
	}

	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Left: Gutter, Right: Gutter, Top: 14, Bottom: 14}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return Editor(gtx, k, ed, o.Hint, o.Mono)
	})
	call := rec.Stop()

	k.FieldArea(gtx, ed, dims.Size)
	call.Add(gtx.Ops)
	return dims
}

func ActionRow(gtx C, k *kit.Kit, clk *widget.Clickable, ic *icons.Icon, label string, kind Kind) D {
	th := k.Th
	fg := th.Accent

	switch kind {
	case Destructive:
		fg = th.Danger
	case Secondary, Quiet:
		fg = th.Text
	}

	return clk.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(FormRowHeight))
		rec := op.Record(gtx.Ops)
		gtx.Constraints = layout.Exact(size)
		layout.Inset{Left: Gutter, Right: Gutter}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if ic == nil {
						return D{}
					}

					return layout.Inset{Right: 12}.Layout(gtx, func(gtx C) D { return Tile(gtx, ic, fg, color.NRGBA{}) })
				}),
				layout.Flexed(1, func(gtx C) D {
					gtx.Constraints.Min.Y = size.Y
					return layout.W.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: TextBody, Color: fg, MaxLines: 1}, label)
					})
				}),
			)
		})
		call := rec.Stop()

		if clk.Pressed() {
			kit.Rect(gtx, image.Rectangle{Max: size}, th.Pressed)
		}

		call.Add(gtx.Ops)
		return D{Size: size}
	})
}

func Heading(gtx C, k *kit.Kit, caption string) D {
	return layout.Inset{Left: 4, Right: 4, Bottom: 8}.Layout(gtx, func(gtx C) D { return heading(gtx, k, caption, "") })
}
