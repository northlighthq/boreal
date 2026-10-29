package kit

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/ui/icons"
)

type ButtonKind uint8

const (
	Primary ButtonKind = iota
	Secondary
	Ghost
	Danger
)

const (
	buttonHeight = unit.Dp(36)
	buttonPadX   = unit.Dp(16)
	iconButton   = unit.Dp(30)
	iconSize     = unit.Dp(18)
)

func (k *Kit) Button(gtx C, clk *widget.Clickable, kind ButtonKind, ic *icons.Icon, label string) D {
	th := k.Th
	h := k.Hover(gtx, Key(clk, keyHover), clk.Hovered())
	p := k.Animated(gtx, Key(clk, keyPress), B2F(clk.Pressed()), Fast)

	var bg, fg, line color.NRGBA

	switch kind {
	case Primary:
		bg, fg = Lerp(th.Accent, th.AccentHover, h), th.OnAccent
	case Danger:
		bg, fg, line = Fade(th.Danger, 0.12+0.1*h), th.Danger, Fade(th.Danger, 0.5)
	case Secondary:
		bg, fg, line = Fade(th.Hover, h), th.Text, th.FieldLine
	case Ghost:
		bg, fg = Fade(th.Hover, h), Lerp(th.TextMuted, th.Text, h)
	}

	bg = Lerp(bg, th.Pressed, p*0.35)

	weight := font.Medium

	if kind == Primary {
		weight = font.Bold
	}

	return clk.Layout(gtx, func(gtx C) D {
		height := gtx.Dp(buttonHeight)
		gtx.Constraints.Min.Y = height

		rec := op.Record(gtx.Ops)
		dims := Background(gtx, bg, buttonHeight/2, func(gtx C) D {
			gtx.Constraints.Min.Y = height
			return layout.Inset{Left: buttonPadX, Right: buttonPadX}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceSides}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if ic == nil {
							return D{}
						}

						return layout.Inset{Right: 7}.Layout(gtx, func(gtx C) D {
							return ic.Layout(gtx, 16, fg)
						})
					}),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.Y = height
						return layout.Center.Layout(gtx, func(gtx C) D {
							return k.Label(gtx, TextStyle{Color: fg, Weight: weight}, label)
						})
					}),
				)
			})
		})

		if line.A > 0 {
			Border(gtx, image.Rectangle{Max: dims.Size}, height/2, 1, line)
		}

		call := rec.Stop()
		call.Add(gtx.Ops)
		Cursor(gtx, dims.Size, pointer.CursorPointer)
		return dims
	})
}

func (k *Kit) IconButton(gtx C, clk *widget.Clickable, ic *icons.Icon, active bool) D {
	return k.IconButtonSized(gtx, clk, ic, active, iconButton, iconSize, k.Th.TextMuted)
}

func (k *Kit) IconButtonSized(gtx C, clk *widget.Clickable, ic *icons.Icon, active bool, box, size unit.Dp, fg color.NRGBA) D {
	th := k.Th
	h := k.Hover(gtx, Key(clk, keyHover), clk.Hovered())
	a := k.Animated(gtx, Key(clk, keyActive), B2F(active), Normal)

	bg := Lerp(Fade(th.Hover, h), th.Selection, a)
	fg = Lerp(Lerp(fg, th.Text, h), th.Text, a)

	return clk.Layout(gtx, func(gtx C) D {
		px := gtx.Dp(box)
		sz := image.Pt(px, px)

		if bg.A > 0 {
			RRect(gtx, image.Rectangle{Max: sz}, gtx.Dp(th.Radius), bg)
		}

		gtx.Constraints = layout.Exact(sz)
		layout.Center.Layout(gtx, func(gtx C) D {
			return ic.Layout(gtx, size, fg)
		})
		Cursor(gtx, sz, pointer.CursorPointer)
		return D{Size: sz}
	})
}

func (k *Kit) Link(gtx C, clk *widget.Clickable, label string) D {
	h := k.Hover(gtx, Key(clk, keyHover), clk.Hovered())
	return clk.Layout(gtx, func(gtx C) D {
		c := Lerp(k.Th.TextMuted, k.Th.Text, h)
		dims := k.Label(gtx, TextStyle{Color: c, Weight: font.Medium}, label)
		Rect(gtx, image.Rect(0, dims.Size.Y-1, dims.Size.X, dims.Size.Y), Lerp(k.Th.FieldLine, k.Th.Text, h))
		Cursor(gtx, dims.Size, pointer.CursorPointer)
		return dims
	})
}
