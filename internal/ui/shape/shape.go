package shape

import (
	"image"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func Ring(ops *op.Ops, r image.Rectangle, radius, width float32) clip.PathSpec {
	x0, y0, x1, y1 := float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)
	var p clip.Path
	p.Begin(ops)
	roundRect(&p, x0, y0, x1, y1, radius, false)
	roundRect(&p, x0+width, y0+width, x1-width, y1-width, max(radius-width, 0), true)
	return p.End()
}

func roundRect(p *clip.Path, x0, y0, x1, y1, rad float32, reverse bool) {
	if x1 <= x0 || y1 <= y0 {
		return
	}

	rad = min(rad, (x1-x0)/2, (y1-y0)/2)
	c := rad * 0.4477
	pt := f32.Pt

	if !reverse {
		p.MoveTo(pt(x0+rad, y0))
		p.LineTo(pt(x1-rad, y0))
		p.CubeTo(pt(x1-c, y0), pt(x1, y0+c), pt(x1, y0+rad))
		p.LineTo(pt(x1, y1-rad))
		p.CubeTo(pt(x1, y1-c), pt(x1-c, y1), pt(x1-rad, y1))
		p.LineTo(pt(x0+rad, y1))
		p.CubeTo(pt(x0+c, y1), pt(x0, y1-c), pt(x0, y1-rad))
		p.LineTo(pt(x0, y0+rad))
		p.CubeTo(pt(x0, y0+c), pt(x0+c, y0), pt(x0+rad, y0))
	} else {
		p.MoveTo(pt(x0+rad, y0))
		p.CubeTo(pt(x0+c, y0), pt(x0, y0+c), pt(x0, y0+rad))
		p.LineTo(pt(x0, y1-rad))
		p.CubeTo(pt(x0, y1-c), pt(x0+c, y1), pt(x0+rad, y1))
		p.LineTo(pt(x1-rad, y1))
		p.CubeTo(pt(x1-c, y1), pt(x1, y1-c), pt(x1, y1-rad))
		p.LineTo(pt(x1, y0+rad))
		p.CubeTo(pt(x1, y0+c), pt(x1-c, y0), pt(x1-rad, y0))
	}

	p.Close()
}
