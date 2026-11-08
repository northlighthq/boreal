package icons

import (
	"math"
	"strconv"

	"gioui.org/f32"
)

type segment struct {
	kind byte
	pts  [3]f32.Point
}

func parsePath(d string) []segment {
	var out []segment
	sc := pathScanner{s: d}
	var cur, start, ctrl f32.Point
	var cmd, prev byte

	for last := -1; sc.i != last; {
		last = sc.i

		if c, ok := sc.command(); ok {
			cmd = c
		} else if !sc.more() {
			break
		}

		rel := cmd >= 'a'
		abs := func(p f32.Point) f32.Point {
			if rel {
				return p.Add(cur)
			}

			return p
		}
		up := cmd &^ 0x20

		switch up {
		case 'M':
			cur = abs(sc.point())
			start = cur
			out = append(out, segment{kind: 'M', pts: [3]f32.Point{cur}})

			if rel {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L':
			cur = abs(sc.point())
			out = append(out, segment{kind: 'L', pts: [3]f32.Point{cur}})
		case 'H':
			x := sc.number()

			if rel {
				x += cur.X
			}

			cur.X = x
			out = append(out, segment{kind: 'L', pts: [3]f32.Point{cur}})
		case 'V':
			y := sc.number()

			if rel {
				y += cur.Y
			}

			cur.Y = y
			out = append(out, segment{kind: 'L', pts: [3]f32.Point{cur}})
		case 'C':
			c1, c2, end := abs(sc.point()), abs(sc.point()), abs(sc.point())
			out = append(out, segment{kind: 'C', pts: [3]f32.Point{c1, c2, end}})
			ctrl, cur = c2, end
		case 'S':
			c1 := cur

			if p := prev &^ 0x20; p == 'C' || p == 'S' {
				c1 = cur.Mul(2).Sub(ctrl)
			}

			c2, end := abs(sc.point()), abs(sc.point())
			out = append(out, segment{kind: 'C', pts: [3]f32.Point{c1, c2, end}})
			ctrl, cur = c2, end
		case 'Q', 'T':
			var q f32.Point

			if up == 'Q' {
				q = abs(sc.point())
			} else if p := prev &^ 0x20; p == 'Q' || p == 'T' {
				q = cur.Mul(2).Sub(ctrl)
			} else {
				q = cur
			}

			end := abs(sc.point())
			out = append(out, quadToCubic(cur, q, end))
			ctrl, cur = q, end
		case 'A':
			rx, ry := sc.number(), sc.number()
			rot := sc.number()
			large, sweep := sc.flag(), sc.flag()
			end := abs(sc.point())
			out = append(out, arcToCubics(cur, end, rx, ry, rot, large, sweep)...)
			cur = end
		case 'Z':
			out = append(out, segment{kind: 'Z'})
			cur = start
		default:
			return out
		}

		prev = cmd
	}

	return out
}

func quadToCubic(p0, q, p1 f32.Point) segment {
	c1 := p0.Add(q.Sub(p0).Mul(2.0 / 3))
	c2 := p1.Add(q.Sub(p1).Mul(2.0 / 3))
	return segment{kind: 'C', pts: [3]f32.Point{c1, c2, p1}}
}

func arcToCubics(p0, p1 f32.Point, rx, ry, rotDeg float32, large, sweep bool) []segment {
	if p0 == p1 {
		return nil
	}

	if rx == 0 || ry == 0 {
		return []segment{{kind: 'L', pts: [3]f32.Point{p1}}}
	}

	x0, y0, x1, y1 := float64(p0.X), float64(p0.Y), float64(p1.X), float64(p1.Y)
	a, b := math.Abs(float64(rx)), math.Abs(float64(ry))
	phi := float64(rotDeg) * math.Pi / 180
	cp, sp := math.Cos(phi), math.Sin(phi)

	dx, dy := (x0-x1)/2, (y0-y1)/2
	x1p := cp*dx + sp*dy
	y1p := -sp*dx + cp*dy

	if l := x1p*x1p/(a*a) + y1p*y1p/(b*b); l > 1 {
		a *= math.Sqrt(l)
		b *= math.Sqrt(l)
	}

	num := a*a*b*b - a*a*y1p*y1p - b*b*x1p*x1p
	den := a*a*y1p*y1p + b*b*x1p*x1p
	co := math.Sqrt(math.Max(0, num/den))

	if large == sweep {
		co = -co
	}

	cxp, cyp := co*a*y1p/b, -co*b*x1p/a
	cx := cp*cxp - sp*cyp + (x0+x1)/2
	cy := sp*cxp + cp*cyp + (y0+y1)/2

	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t1 := angle(1, 0, (x1p-cxp)/a, (y1p-cyp)/b)
	dt := angle((x1p-cxp)/a, (y1p-cyp)/b, (-x1p-cxp)/a, (-y1p-cyp)/b)

	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}

	n := int(math.Ceil(math.Abs(dt) / (math.Pi / 2)))
	step := dt / float64(n)
	alpha := 4.0 / 3 * math.Tan(step/4)
	at := func(t float64) (x, y, ddx, ddy float64) {
		ct, st := math.Cos(t), math.Sin(t)
		x = cx + a*ct*cp - b*st*sp
		y = cy + a*ct*sp + b*st*cp
		ddx = -a*st*cp - b*ct*sp
		ddy = -a*st*sp + b*ct*cp
		return
	}

	out := make([]segment, 0, n)
	t := t1

	for i := 0; i < n; i++ {
		ax, ay, adx, ady := at(t)
		bx, by, bdx, bdy := at(t + step)
		c1 := f32.Pt(float32(ax+alpha*adx), float32(ay+alpha*ady))
		c2 := f32.Pt(float32(bx-alpha*bdx), float32(by-alpha*bdy))
		end := f32.Pt(float32(bx), float32(by))

		if i == n-1 {
			end = p1
		}

		out = append(out, segment{kind: 'C', pts: [3]f32.Point{c1, c2, end}})
		t += step
	}

	return out
}

type pathScanner struct {
	s string
	i int
}

func (sc *pathScanner) skip() {
	for sc.i < len(sc.s) {
		switch sc.s[sc.i] {
		case ' ', ',', '\t', '\n', '\r':
			sc.i++
		default:
			return
		}
	}
}

func (sc *pathScanner) more() bool {
	sc.skip()
	return sc.i < len(sc.s)
}

func (sc *pathScanner) command() (byte, bool) {
	sc.skip()

	if sc.i >= len(sc.s) {
		return 0, false
	}

	c := sc.s[sc.i]

	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		if c == 'e' || c == 'E' {
			return 0, false
		}

		sc.i++
		return c, true
	}

	return 0, false
}

func (sc *pathScanner) number() float32 {
	sc.skip()
	j := sc.i

	if j < len(sc.s) && (sc.s[j] == '-' || sc.s[j] == '+') {
		j++
	}

	dot := false

	for j < len(sc.s) {
		c := sc.s[j]

		switch {
		case c >= '0' && c <= '9':
		case c == '.' && !dot:
			dot = true
		case c == 'e' || c == 'E':
			j++

			if j < len(sc.s) && (sc.s[j] == '-' || sc.s[j] == '+') {
				j++
			}

			continue
		default:
			goto done
		}

		j++
	}

done:
	v, _ := strconv.ParseFloat(sc.s[sc.i:j], 32)
	sc.i = j
	return float32(v)
}

func (sc *pathScanner) flag() bool {
	sc.skip()

	if sc.i < len(sc.s) {
		c := sc.s[sc.i]
		sc.i++
		return c == '1'
	}

	return false
}

func (sc *pathScanner) point() f32.Point {
	x := sc.number()
	y := sc.number()
	return f32.Pt(x, y)
}
