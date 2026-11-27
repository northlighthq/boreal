package settings

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const (
	previewPad    = unit.Dp(16)
	cursorWidth   = 0.6
	cursorHeight  = 1.25
	cursorThin    = unit.Dp(2)
	ansiGreen     = 10
	ansiBlue      = 12
	previewUser   = "you@boreal"
	previewListed = "app  logs  www"
)

func termPreview(gtx C, k *kit.Kit, s config.Settings) D {
	th := k.Th
	size := unit.Sp(s.FontSize)
	style := func(i int) kit.TextStyle {
		c := th.Foreground

		if i >= 0 {
			c = th.ANSI[i]
		}

		return kit.TextStyle{Size: size, Color: c, Mono: true, Face: font.Typeface(fonts.TerminalFamily(s.TerminalFont)), MaxLines: 1}
	}

	prompt := func(rest ...layout.FlexChild) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			parts := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return k.Label(gtx, style(ansiGreen), previewUser) }),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, style(-1), ":") }),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, style(ansiBlue), "~") }),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, style(-1), "$ ") }),
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, append(parts, rest...)...)
		})
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return terminalCard(gtx, k, func(gtx C) D {
		return layout.UniformInset(previewPad).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				prompt(layout.Rigid(func(gtx C) D { return k.Label(gtx, style(-1), "ls") })),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, style(ansiBlue), previewListed) }),
				prompt(layout.Rigid(func(gtx C) D { return cursor(gtx, k, size, s.CursorStyle) })),
			)
		})
	})
}

func terminalCard(gtx C, k *kit.Kit, w ui.W) D {
	th := k.Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	dims := w(gtx)
	call := rec.Stop()
	r := image.Rectangle{Max: dims.Size}
	radius := gtx.Dp(th.RadiusLarge)
	kit.RRect(gtx, r, radius, th.Background)
	kit.Border(gtx, r, radius, 1, th.Divider)
	call.Add(gtx.Ops)
	return dims
}

func cursor(gtx C, k *kit.Kit, size unit.Sp, style string) D {
	w := int(float32(gtx.Sp(size)) * cursorWidth)
	h := int(float32(gtx.Sp(size)) * cursorHeight)
	thin := gtx.Dp(cursorThin)
	r := image.Rect(0, 0, w, h)

	switch style {
	case config.CursorBar:
		r.Max.X = thin
	case config.CursorUnderline:
		r.Min.Y = h - thin
	}

	defer clip.Rect(r).Push(gtx.Ops).Pop()
	kit.Rect(gtx, r, k.Th.Cursor)
	return D{Size: image.Pt(w, h)}
}
