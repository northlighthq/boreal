package icons

import (
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const tablerStroke = 1.75

func tabler(name string) *Icon {
	return tablerIcon("outline-"+name, false)
}

func lucide(name string) *Icon {
	return tablerIcon("lucide-"+name, false)
}

func tablerFilled(name string) *Icon {
	return tablerIcon("filled-"+name, true)
}

func tablerIcon(key string, fill bool) *Icon {
	d, ok := tablerPaths[key]

	if !ok {
		d, ok = lucidePaths[key]
	}

	if !ok {
		panic("icons: no such icon " + key)
	}

	segs := parsePath(d)

	if !fill {
		segs = outline(segs, tablerStroke/2)
	}

	return custom(func(ops *op.Ops, s float32, c color.NRGBA) {
		paint.FillShape(ops, c, clip.Outline{Path: build(ops, segs, s/24)}.Op())
	})
}

func build(ops *op.Ops, segs []segment, k float32) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)

	for _, sg := range segs {
		switch sg.kind {
		case 'M':
			p.MoveTo(sg.pts[0].Mul(k))
		case 'L':
			p.LineTo(sg.pts[0].Mul(k))
		case 'C':
			p.CubeTo(sg.pts[0].Mul(k), sg.pts[1].Mul(k), sg.pts[2].Mul(k))
		case 'Z':
			p.Close()
		}
	}

	return p.End()
}

func outline(segs []segment, r float32) []segment {
	var out []segment
	var lines [][]f32.Point
	var cur, start f32.Point

	for _, sg := range segs {
		switch sg.kind {
		case 'M':
			cur, start = sg.pts[0], sg.pts[0]
			lines = append(lines, []f32.Point{cur})
		case 'L':
			cur = sg.pts[0]
			lines[len(lines)-1] = append(lines[len(lines)-1], cur)
		case 'C':
			p0 := cur

			for i := 1; i <= 10; i++ {
				t := float32(i) / 10
				u := 1 - t
				pt := p0.Mul(u * u * u).Add(sg.pts[0].Mul(3 * u * u * t)).Add(sg.pts[1].Mul(3 * u * t * t)).Add(sg.pts[2].Mul(t * t * t))
				lines[len(lines)-1] = append(lines[len(lines)-1], pt)
			}

			cur = sg.pts[2]
		case 'Z':
			lines[len(lines)-1] = append(lines[len(lines)-1], start)
			cur = start
		}
	}

	for _, ln := range lines {
		for i, p := range ln {
			if i == 0 || i == len(ln)-1 || corner(ln[i-1], p, ln[i+1]) {
				out = append(out, disc(p, r)...)
			}

			if i == 0 {
				continue
			}

			q := ln[i-1]
			dx, dy := p.X-q.X, p.Y-q.Y
			l := float32(math.Hypot(float64(dx), float64(dy)))

			if l < 1e-4 {
				continue
			}

			n := f32.Pt(-dy/l*r, dx/l*r)
			out = append(out,
				segment{kind: 'M', pts: [3]f32.Point{q.Add(n)}},
				segment{kind: 'L', pts: [3]f32.Point{q.Sub(n)}},
				segment{kind: 'L', pts: [3]f32.Point{p.Sub(n)}},
				segment{kind: 'L', pts: [3]f32.Point{p.Add(n)}},
				segment{kind: 'Z'},
			)
		}
	}

	return out
}

func corner(a, b, c f32.Point) bool {
	u, v := b.Sub(a), c.Sub(b)
	lu := math.Hypot(float64(u.X), float64(u.Y))
	lv := math.Hypot(float64(v.X), float64(v.Y))

	if lu < 1e-4 || lv < 1e-4 {
		return true
	}

	return float64(u.X*v.X+u.Y*v.Y)/(lu*lv) < 0.995
}

func disc(c f32.Point, r float32) []segment {
	k := r * 0.5522847
	pt := func(x, y float32) f32.Point { return f32.Pt(c.X+x, c.Y+y) }

	return []segment{
		{kind: 'M', pts: [3]f32.Point{pt(r, 0)}},
		{kind: 'C', pts: [3]f32.Point{pt(r, k), pt(k, r), pt(0, r)}},
		{kind: 'C', pts: [3]f32.Point{pt(-k, r), pt(-r, k), pt(-r, 0)}},
		{kind: 'C', pts: [3]f32.Point{pt(-r, -k), pt(-k, -r), pt(0, -r)}},
		{kind: 'C', pts: [3]f32.Point{pt(k, -r), pt(r, -k), pt(r, 0)}},
		{kind: 'Z'},
	}
}
