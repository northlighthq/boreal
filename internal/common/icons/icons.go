package icons

import (
	"image"
	"image/color"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/common/shape"
)

type Icon struct {
	draw  func(ops *op.Ops, size float32, c color.NRGBA)
	shape func(ops *op.Ops, size float32) clip.PathSpec

	mu     sync.Mutex
	shapes map[int]shapeCache
}

type shapeCache struct {
	ops  *op.Ops
	call op.CallOp
}

func custom(fn func(ops *op.Ops, size float32, c color.NRGBA)) *Icon {
	return &Icon{draw: fn}
}

func outlined(fn func(ops *op.Ops, size float32) clip.PathSpec) *Icon {
	return &Icon{shape: fn}
}

func (i *Icon) Layout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))

	if i.shape == nil {
		i.draw(gtx.Ops, float32(px), c)
		return layout.Dimensions{Size: image.Pt(px, px)}
	}

	paint.ColorOp{Color: c}.Add(gtx.Ops)
	i.cached(px).Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(px, px)}
}

func (i *Icon) cached(px int) op.CallOp {
	i.mu.Lock()
	defer i.mu.Unlock()

	if sc, ok := i.shapes[px]; ok {
		return sc.call
	}

	if i.shapes == nil {
		i.shapes = map[int]shapeCache{}
	}

	ops := new(op.Ops)
	m := op.Record(ops)
	area := clip.Outline{Path: i.shape(ops, float32(px))}.Op().Push(ops)
	paint.PaintOp{}.Add(ops)
	area.Pop()
	sc := shapeCache{ops: ops, call: m.Stop()}
	i.shapes[px] = sc
	return sc.call
}

var (
	SidebarLeft  = sidebar(false)
	SidebarRight = sidebar(true)
)

func sidebar(right bool) *Icon {
	return custom(func(ops *op.Ops, s float32, c color.NRGBA) {
		stroke := max(s/14, 1.2)
		r := image.Rect(int(s*0.12), int(s*0.2), int(s*0.88), int(s*0.8))
		paint.FillShape(ops, c, clip.Outline{Path: shape.Ring(ops, r, float32(int(s*0.1)), stroke)}.Op())

		col := image.Rect(r.Min.X, r.Min.Y, r.Min.X+int(s*0.28), r.Max.Y)

		if right {
			col = image.Rect(r.Max.X-int(s*0.28), r.Min.Y, r.Max.X, r.Max.Y)
		}

		paint.FillShape(ops, c, clip.UniformRRect(col, int(s*0.1)).Op(ops))
	})
}

var Minimize = custom(func(ops *op.Ops, s float32, c color.NRGBA) {
	y := int(s / 2)
	paint.FillShape(ops, c, clip.Rect{Min: image.Pt(int(s*0.3), y), Max: image.Pt(int(s*0.7), y+1)}.Op())
})

var Maximize = custom(func(ops *op.Ops, s float32, c color.NRGBA) {
	r := image.Rect(int(s*0.3), int(s*0.3), int(s*0.7), int(s*0.7))
	strokeRect(ops, r, c)
})

var Restore = custom(func(ops *op.Ops, s float32, c color.NRGBA) {
	back := image.Rect(int(s*0.38), int(s*0.26), int(s*0.74), int(s*0.62))
	front := image.Rect(int(s*0.26), int(s*0.38), int(s*0.62), int(s*0.74))
	paint.FillShape(ops, c, clip.Rect{Min: back.Min, Max: image.Pt(back.Max.X, back.Min.Y+1)}.Op())
	paint.FillShape(ops, c, clip.Rect{Min: image.Pt(back.Max.X-1, back.Min.Y), Max: image.Pt(back.Max.X, back.Max.Y)}.Op())
	strokeRect(ops, front, c)
})

var WindowClose = custom(func(ops *op.Ops, s float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(s*0.3, s*0.3))
	p.LineTo(f32.Pt(s*0.7, s*0.7))
	p.MoveTo(f32.Pt(s*0.7, s*0.3))
	p.LineTo(f32.Pt(s*0.3, s*0.7))
	paint.FillShape(ops, c, clip.Stroke{Path: p.End(), Width: 1}.Op())
})

func strokeRect(ops *op.Ops, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(ops, c, clip.Rect{Min: r.Min, Max: image.Pt(r.Max.X, r.Min.Y+1)}.Op())
	paint.FillShape(ops, c, clip.Rect{Min: image.Pt(r.Min.X, r.Max.Y-1), Max: r.Max}.Op())
	paint.FillShape(ops, c, clip.Rect{Min: r.Min, Max: image.Pt(r.Min.X+1, r.Max.Y)}.Op())
	paint.FillShape(ops, c, clip.Rect{Min: image.Pt(r.Max.X-1, r.Min.Y), Max: r.Max}.Op())
}

var MoreH = custom(func(ops *op.Ops, s float32, c color.NRGBA) {
	d := max(int(s*0.16+0.5), 3)
	gap := max(int(s*0.12+0.5), 2)
	x := (int(s) - 3*d - 2*gap) / 2
	y := (int(s) - d) / 2

	for i := 0; i < 3; i++ {
		r := image.Rect(x, y, x+d, y+d)
		paint.FillShape(ops, c, clip.Ellipse(r).Op(ops))
		x += d + gap
	}
})
