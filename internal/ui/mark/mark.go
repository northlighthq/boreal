package mark

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

var (
	Aurora = color.NRGBA{R: 0x3B, G: 0xD3, B: 0x9A, A: 0xFF}
	shapes = [][]float32{
		{0, 48, 96, 48, 96, 208, 160, 208, 160, 256, 0, 256},
		{96, 0, 256, 0, 256, 208, 160, 208, 160, 48, 96, 48},
	}
)

func Layout(gtx layout.Context, size unit.Dp) layout.Dimensions {
	return Paint(gtx, size, Aurora)
}

func Paint(gtx layout.Context, size unit.Dp, col color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	scale := float32(px) / 256

	for _, shape := range shapes {
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(f32.Pt(shape[0]*scale, shape[1]*scale))
		for i := 2; i < len(shape); i += 2 {
			p.LineTo(f32.Pt(shape[i]*scale, shape[i+1]*scale))
		}
		p.Close()
		paint.FillShape(gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
	}

	return layout.Dimensions{Size: image.Pt(px, px)}
}
