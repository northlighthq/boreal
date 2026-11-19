package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/theme"
)

type Kind uint8

const (
	Primary Kind = iota
	Secondary
	Destructive
	Quiet
)

type FieldOpts struct {
	Label    string
	Hint     string
	Help     string
	Mono     bool
	Invalid  bool
	Trailing W
}

func SetupField(ed *widget.Editor, hint key.InputHint, secret bool) {
	ed.SingleLine = true
	ed.Submit = true
	ed.InputHint = hint

	if secret {
		ed.Mask = '•'
	}
}

func Field(gtx C, k *kit.Kit, ed *widget.Editor, o FieldOpts) D {
	th := k.Th
	children := []layout.FlexChild{}

	if o.Label != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, MaxLines: 1}, o.Label)
			})
		}))
	}

	children = append(children, layout.Rigid(func(gtx C) D { return fieldBox(gtx, k, ed, o) }))

	if o.Help != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			c := th.TextMuted

			if o.Invalid {
				c = th.Danger
			}

			return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: TextSub - 1, Color: c, MaxLines: 3}, o.Help)
			})
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func fieldBox(gtx C, k *kit.Kit, ed *widget.Editor, o FieldOpts) D {
	th := k.Th
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(FieldHeight))

	if !ed.SingleLine {
		size.Y = gtx.Dp(132)
	}

	r := image.Rectangle{Max: size}
	radius := gtx.Dp(th.Radius)
	focus := k.Animated(gtx, kit.Key(ed, 0xe1), kit.B2F(gtx.Focused(ed)), kit.Normal)
	kit.RRect(gtx, r, radius, th.Field)
	line := kit.Lerp(th.FieldLine, th.TextFaint, focus)

	if o.Invalid {
		line = th.Danger
	}

	kit.Border(gtx, r, radius, 1, line)
	k.FieldArea(gtx, ed, size)
	gtx.Constraints = layout.Exact(size)

	return layout.Inset{Left: 12, Right: 4}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				if ed.SingleLine {
					return centerY(gtx, func(gtx C) D { return Editor(gtx, k, ed, o.Hint, o.Mono) })
				}

				return layout.Inset{Top: 12, Bottom: 12}.Layout(gtx, func(gtx C) D {
					return layout.NW.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return Editor(gtx, k, ed, o.Hint, o.Mono)
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				if o.Trailing == nil {
					return D{}
				}

				return o.Trailing(gtx)
			}),
		)
	})
}

func Editor(gtx C, k *kit.Kit, ed *widget.Editor, hint string, mono bool) D {
	st := material.Editor(k.Mat, ed, hint)
	st.Font = kit.Font(font.Normal)

	if mono {
		st.Font = font.Font{Typeface: fonts.Mono}
	}

	st.TextSize = TextBody
	st.Color = k.Th.Text
	st.HintColor = k.Th.TextFaint
	st.SelectionColor = theme.WithAlpha(k.Th.Text, 0x30)
	return st.Layout(gtx)
}

func Search(gtx C, k *kit.Kit, ed *widget.Editor, hint string) D {
	th := k.Th
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(44))
	r := image.Rectangle{Max: size}
	radius := gtx.Dp(th.Radius)
	kit.RRect(gtx, r, radius, th.Field)
	kit.Border(gtx, r, radius, 1, th.FieldLine)
	k.FieldArea(gtx, ed, size)
	gtx.Constraints = layout.Exact(size)

	return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icons.Search.Layout(gtx, 18, th.TextMuted) }),
			layout.Rigid(kit.Gap(10)),
			layout.Flexed(1, func(gtx C) D {
				return centerY(gtx, func(gtx C) D { return Editor(gtx, k, ed, hint, false) })
			}),
		)
	})
}

func Button(gtx C, k *kit.Kit, clk *widget.Clickable, kind Kind, ic *icons.Icon, label string, enabled bool) D {
	th := k.Th
	bg, fg, line := th.Accent, th.OnAccent, color.NRGBA{}
	weight := font.Bold

	switch kind {
	case Secondary:
		bg, fg, line, weight = color.NRGBA{}, th.Text, th.FieldLine, font.Medium
	case Destructive:
		bg, fg, line, weight = kit.Fade(th.Danger, 0.12), th.Danger, kit.Fade(th.Danger, 0.5), font.Medium
	case Quiet:
		bg, fg, weight = color.NRGBA{}, th.TextMuted, font.Medium
	}

	if !enabled {
		bg, fg = th.Field, th.TextFaint
		gtx = gtx.Disabled()
	}

	return clk.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(ButtonHeight))
		r := image.Rectangle{Max: size}
		radius := size.Y / 2

		if clk.Pressed() {
			bg = kit.Lerp(bg, th.Pressed, 0.35)

			if kind == Primary {
				bg = th.AccentHover
			}
		}

		if bg.A > 0 {
			kit.RRect(gtx, r, radius, bg)
		}

		if line.A > 0 && enabled {
			kit.Border(gtx, r, radius, 1, line)
		}

		gtx.Constraints = layout.Exact(size)
		return layout.Center.Layout(gtx, func(gtx C) D { return iconLabel(gtx, k, ic, label, fg, weight) })
	})
}

func iconLabel(gtx C, k *kit.Kit, ic *icons.Icon, label string, fg color.NRGBA, w font.Weight) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if ic == nil {
				return D{}
			}

			return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 18, fg) })
		}),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: TextBody, Color: fg, Weight: w, MaxLines: 1}, label)
		}),
	)
}

func IconButton(gtx C, k *kit.Kit, clk *widget.Clickable, ic *icons.Icon, fg color.NRGBA) D {
	th := k.Th

	return clk.Layout(gtx, func(gtx C) D {
		side := gtx.Dp(MinTouch)
		size := image.Pt(side, side)

		if clk.Pressed() {
			kit.RRect(gtx, image.Rectangle{Max: size}, side/2, th.Pressed)
		}

		gtx.Constraints = layout.Exact(size)
		return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 20, fg) })
	})
}

type Segmented struct {
	Value int
	clks  []widget.Clickable
}

func (s *Segmented) Changed(gtx C) bool {
	changed := false

	for i := range s.clks {
		if s.clks[i].Clicked(gtx) && s.Value != i {
			s.Value = i
			changed = true
		}
	}

	return changed
}

func (s *Segmented) Layout(gtx C, k *kit.Kit, labels []string) D {
	th := k.Th

	if len(s.clks) != len(labels) {
		s.clks = make([]widget.Clickable, len(labels))
	}

	s.Changed(gtx)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(FieldHeight))
	r := image.Rectangle{Max: size}
	radius := gtx.Dp(th.Radius)
	kit.RRect(gtx, r, radius, th.Field)
	kit.Border(gtx, r, radius, 1, th.FieldLine)
	gtx.Constraints = layout.Exact(size)
	children := make([]layout.FlexChild, len(labels))

	for i, l := range labels {
		children[i] = layout.Flexed(1, func(gtx C) D {
			return s.clks[i].Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				on := k.Animated(gtx, kit.Key(&s.clks[i], 0xe2), kit.B2F(s.Value == i), kit.Normal)

				if on > 0 {
					in := image.Rectangle{Max: gtx.Constraints.Max}.Inset(gtx.Dp(3))
					kit.RRect(gtx, in, radius-gtx.Dp(3), kit.Fade(th.Selection, on))
					kit.Border(gtx, in, radius-gtx.Dp(3), 1, kit.Fade(th.FieldLine, on))
				}

				return layout.Center.Layout(gtx, func(gtx C) D {
					fg := kit.Lerp(th.TextMuted, th.Text, on)
					return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: fg, Weight: font.Medium, MaxLines: 1}, l)
				})
			})
		})
	}

	return layout.Flex{}.Layout(gtx, children...)
}

func Gap(dp unit.Dp) W { return kit.Gap(dp) }

func Fill(gtx C, c color.NRGBA) {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	kit.Rect(gtx, image.Rectangle{Max: gtx.Constraints.Max}, c)
}

func centerY(gtx C, w W) D {
	h := gtx.Constraints.Max.Y
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(gtx.Constraints.Max.X, 0), Max: image.Pt(gtx.Constraints.Max.X, h)}
	rec := op.Record(gtx.Ops)
	dims := w(g)
	call := rec.Stop()
	defer op.Offset(image.Pt(0, (h-dims.Size.Y)/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
}
