package mark

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
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

func Stage(gtx layout.Context, box, size unit.Dp, col color.NRGBA, t float64, bounce bool) layout.Dimensions {
	b := gtx.Dp(box)
	px := gtx.Dp(size)
	lift := 0

	if bounce {
		lift = int(math.Abs(math.Sin(t*math.Pi*1.4)) * float64(gtx.Dp(10)))
		col.A = uint8(float64(col.A) * (0.75 + 0.25*math.Cos(t*math.Pi*1.4*2)))
	}

	defer op.Offset(image.Pt((b-px)/2, (b-px)/2-lift)).Push(gtx.Ops).Pop()
	Paint(gtx, size, col)
	gtx.Execute(op.InvalidateCmd{})

	return layout.Dimensions{Size: image.Pt(b, b)}
}
