package retro

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const (
	retroSize = unit.Dp(440)

	retroYaw   = -32 * math.Pi / 180
	retroPitch = 22 * math.Pi / 180

	strokeStagger = 12 * time.Millisecond
	strokeLength  = 120 * time.Millisecond

	typeStep  = 45 * time.Millisecond
	blinkStep = 530 * time.Millisecond
)

type vec3 struct{ x, y, z float64 }

type part struct {
	faces   [][]vec3
	strokes [][2]vec3
}

func box(x0, y0, z0, x1, y1, z1 float64) part {
	v := func(x, y, z float64) vec3 { return vec3{x, y, z} }
	front := []vec3{v(x0, y0, z1), v(x1, y0, z1), v(x1, y1, z1), v(x0, y1, z1)}
	right := []vec3{v(x1, y0, z1), v(x1, y0, z0), v(x1, y1, z0), v(x1, y1, z1)}
	top := []vec3{v(x0, y1, z1), v(x1, y1, z1), v(x1, y1, z0), v(x0, y1, z0)}
	return part{
		faces: [][]vec3{front, right, top},
		strokes: [][2]vec3{
			{v(x0, y0, z1), v(x1, y0, z1)}, {v(x1, y0, z1), v(x1, y1, z1)},
			{v(x1, y1, z1), v(x0, y1, z1)}, {v(x0, y1, z1), v(x0, y0, z1)},
			{v(x1, y0, z1), v(x1, y0, z0)}, {v(x1, y0, z0), v(x1, y1, z0)},
			{v(x1, y1, z0), v(x1, y1, z1)},
			{v(x1, y1, z0), v(x0, y1, z0)}, {v(x0, y1, z0), v(x0, y1, z1)},
		},
	}
}

func rectOnFront(x0, y0, x1, y1, z float64) [][2]vec3 {
	v := func(x, y float64) vec3 { return vec3{x, y, z} }
	return [][2]vec3{{v(x0, y0), v(x1, y0)}, {v(x1, y0), v(x1, y1)}, {v(x1, y1), v(x0, y1)}, {v(x0, y1), v(x0, y0)}}
}

const (
	screenX0, screenX1 = -0.42, 0.42
	screenY0, screenY1 = 0.52, 1.12
	screenZ            = 0.52
)

func computerParts() []part {
	base := box(-0.78, 0, -0.62, 0.78, 0.3, 0.62)
	base.strokes = append(base.strokes,
		[2]vec3{{0.08, 0.2, 0.62}, {0.6, 0.2, 0.62}},
		[2]vec3{{0.08, 0.11, 0.62}, {0.6, 0.11, 0.62}},
		[2]vec3{{0.26, 0.155, 0.62}, {0.42, 0.155, 0.62}},
	)
	base.strokes = append(base.strokes, rectOnFront(-0.62, 0.09, -0.5, 0.21, 0.62)...)

	tube := box(-0.38, 0.42, -0.5, 0.38, 1.08, -0.06)

	monitor := box(-0.58, 0.34, -0.06, 0.58, 1.3, 0.52)
	monitor.strokes = append(monitor.strokes, rectOnFront(screenX0, screenY0, screenX1, screenY1, screenZ)...)
	monitor.strokes = append(monitor.strokes, rectOnFront(screenX0-0.06, screenY0-0.06, screenX1+0.06, screenY1+0.06, screenZ)...)

	for i := 0; i < 4; i++ {
		z := -0.0 + float64(i)*0.1
		monitor.strokes = append(monitor.strokes, [2]vec3{{0.58, 1.16, z}, {0.58, 1.0, z}})
	}

	keyboard := box(kbX0, 0, kbZ0, kbX1, kbTop, kbZ1)

	return []part{base, tube, monitor, keyboard}
}

func project(p vec3) (x, y float64) {
	cy, sy := math.Cos(retroYaw), math.Sin(retroYaw)
	cp, sp := math.Cos(retroPitch), math.Sin(retroPitch)
	xr := p.x*cy + p.z*sy
	zr := -p.x*sy + p.z*cy
	return xr, -p.y*cp + zr*sp
}

type Computer struct {
	click   gesture.Click
	parts   []part
	drawn   time.Time
	hovered bool
	since   time.Time

	keyRects    []keyRect
	hoverKey    *keyRect
	typed       []rune
	lines       []string
	hoverScreen bool
	typing      bool
	shift       bool
	flashKey    *vkey
	flashAt     time.Time
	pointer     f32.Point
	origin      f32.Point
	fit         func(vec3) f32.Point
	command     string
	cache       retroCache
	onRun       func(cmd string)
}

func (r *Computer) Layout(gtx layout.Context, k *kit.Kit, command string) layout.Dimensions {
	th := k.Th

	if r.parts == nil {
		r.parts = computerParts()
	}

	if r.drawn.IsZero() {
		r.drawn = gtx.Now
	}

	if r.keyRects == nil {
		r.keyRects = keyLayout()
	}

	if hv := r.click.Hovered(); hv != r.hovered {
		r.hovered = hv
		r.since = gtx.Now
	}

	on := k.Animated(gtx, kit.Key(r, 0), kit.B2F(r.hovered), kit.Slow)

	{
		side := gtx.Dp(retroSize)
		size := image.Pt(side, side)

		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)

		for _, p := range r.parts {
			for _, s := range p.strokes {
				for _, q := range s {
					x, y := project(q)
					minX, maxX = math.Min(minX, x), math.Max(maxX, x)
					minY, maxY = math.Min(minY, y), math.Max(maxY, y)
				}
			}
		}

		scale := float64(side) * 0.9 / math.Max(maxX-minX, maxY-minY)
		offX := float64(side)/2 - (minX+maxX)/2*scale
		offY := float64(side)/2 - (minY+maxY)/2*scale
		pt := func(p vec3) f32.Point {
			x, y := project(p)
			return f32.Pt(float32(x*scale+offX), float32(y*scale+offY))
		}
		r.fit = pt
		r.command = command
		r.events(k, gtx, pt)

		elapsed := gtx.Now.Sub(r.drawn)
		down := r.pressed(gtx, command)

		if elapsed < r.drawTime() {
			if r.paintBody(gtx, th, pt, on, elapsed) {
				gtx.Execute(op.InvalidateCmd{})
			}
		} else {
			key := retroKey{side: side, scale: gtx.Metric.PxPerDp, on: on, th: th, hover: r.hoverKey, down: down, shift: r.shift}

			if r.flashKey != nil && gtx.Now.Sub(r.flashAt) < keyFlash {
				key.flash = r.flashKey
			}

			c := &r.cache

			if !c.ok || c.key != key {
				c.ops.Reset()
				g := gtx
				g.Ops = &c.ops
				m := op.Record(g.Ops)
				r.paintBody(g, th, pt, on, elapsed)
				r.details(g, th, pt, on)
				r.keys(k, g, pt, on, down)
				c.call, c.key, c.ok = m.Stop(), key, true
			}

			c.call.Add(gtx.Ops)
			r.screenText(k, gtx, pt, on, command)
		}

		area := clip.Rect{Max: size}.Push(gtx.Ops)
		r.click.Add(gtx.Ops)
		event.Op(gtx.Ops, r)

		if r.hoverKey != nil || r.hoverScreen {
			pointer.CursorPointer.Add(gtx.Ops)
		}

		area.Pop()
		return layout.Dimensions{Size: size}
	}
}

type retroKey struct {
	side  int
	scale float32
	on    float32
	th    *theme.Theme
	hover *keyRect
	flash *vkey
	down  rune
	shift bool
}

type retroCache struct {
	ops  op.Ops
	call op.CallOp
	key  retroKey
	ok   bool
}

func (r *Computer) drawTime() time.Duration {
	n := 0

	for _, p := range r.parts {
		n += len(p.strokes)
	}

	return time.Duration(n)*strokeStagger + strokeLength
}

func (r *Computer) paintBody(gtx layout.Context, th *theme.Theme, pt func(vec3) f32.Point, on float32, elapsed time.Duration) bool {
	line := kit.Lerp(kit.Lerp(th.TextFaint, th.TextMuted, 0.6), th.Text, on*0.6)
	width := float32(gtx.Dp(1.25))

	n := 0
	animating := false

	for pi, p := range r.parts {
		start := time.Duration(n) * strokeStagger

		if elapsed < start {
			animating = true
			break
		}

		shades := [3]color.NRGBA{
			kit.Lerp(th.Editor, th.Text, 0.025),
			kit.Lerp(th.Editor, th.Window, 0.55),
			kit.Lerp(th.Editor, th.Text, 0.05),
		}

		for fi, f := range p.faces {
			var path clip.Path
			path.Begin(gtx.Ops)
			path.MoveTo(pt(f[0]))

			for _, q := range f[1:] {
				path.LineTo(pt(q))
			}

			path.Close()
			paint.FillShape(gtx.Ops, shades[fi%3], clip.Outline{Path: path.End()}.Op())
		}

		if pi == 2 && on > 0 {
			r.paintScreen(gtx, pt, kit.Fade(th.Text, 0.07*on))
		}

		for _, s := range p.strokes {
			t := float64(elapsed-time.Duration(n)*strokeStagger) / float64(strokeLength)
			n++

			if t <= 0 {
				animating = true
				continue
			}

			if t < 1 {
				animating = true
			}

			t = math.Min(t, 1)
			p0, p1 := pt(s[0]), pt(s[1])
			end := p0.Add(p1.Sub(p0).Mul(float32(t)))
			var path clip.Path
			path.Begin(gtx.Ops)
			path.MoveTo(p0)
			path.LineTo(end)
			paint.FillShape(gtx.Ops, line, clip.Stroke{Path: path.End(), Width: width}.Op())
		}
	}

	return animating
}

func (r *Computer) paintScreen(gtx layout.Context, pt func(vec3) f32.Point, c color.NRGBA) {
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(pt(vec3{screenX0, screenY0, screenZ}))
	path.LineTo(pt(vec3{screenX1, screenY0, screenZ}))
	path.LineTo(pt(vec3{screenX1, screenY1, screenZ}))
	path.LineTo(pt(vec3{screenX0, screenY1, screenZ}))
	path.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: path.End()}.Op())
}

func (r *Computer) screenText(k *kit.Kit, gtx layout.Context, pt func(vec3) f32.Point, on float32, command string) {
	th := k.Th

	text := ">"
	cursor := true

	if r.typing {
		typed := []rune(strings.Join(append(append([]string{}, r.lines...), "$ "+string(r.typed)), "\n"))

		if len(typed) > 60 {
			typed = typed[len(typed)-60:]
		}

		text = string(typed)
		phase := gtx.Now.Sub(r.flashAt) / blinkStep
		cursor = phase%2 == 0
		gtx.Execute(op.InvalidateCmd{At: r.flashAt.Add((phase + 1) * blinkStep)})
	} else if r.hovered {
		full := "$ " + command
		runes := []rune(full)
		n := min(int(gtx.Now.Sub(r.since)/typeStep), len(runes))
		text = string(runes[:n])

		if n < len(runes) {
			gtx.Execute(op.InvalidateCmd{At: r.since.Add(time.Duration(n+1) * typeStep)})
		} else {
			done := r.since.Add(time.Duration(n) * typeStep)
			phase := gtx.Now.Sub(done) / blinkStep
			cursor = phase%2 == 0
			gtx.Execute(op.InvalidateCmd{At: done.Add((phase + 1) * blinkStep)})
		}
	}

	if cursor {
		text += "_"
	}

	const planeW, planeH = 300, 200
	origin := pt(vec3{screenX0, screenY1, screenZ})
	ex := pt(vec3{screenX1, screenY1, screenZ}).Sub(origin).Mul(1.0 / planeW)
	ey := pt(vec3{screenX0, screenY0, screenZ}).Sub(origin).Mul(1.0 / planeH)
	tr := f32.NewAffine2D(ex.X, ey.X, origin.X, ex.Y, ey.Y, origin.Y)
	defer op.Affine(tr).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: image.Pt(planeW, planeH)}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(18, 16)).Push(gtx.Ops).Pop()

	g := gtx
	g.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
	g.Constraints = layout.Constraints{Max: image.Pt(planeW-36, planeH-32)}
	k.Label(g, kit.TextStyle{Size: 34, Mono: true, Color: kit.Lerp(th.TextMuted, th.Text, on), MaxLines: 4}, text)
}

func (r *Computer) events(k *kit.Kit, gtx layout.Context, pt func(vec3) f32.Point) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: r, Kinds: pointer.Move | pointer.Enter | pointer.Leave})

		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok {
			r.pointer = e.Position
			r.origin = k.Pointer().Sub(e.Position)

			if e.Kind == pointer.Leave {
				r.pointer = f32.Pt(-1, -1)
			}
		}
	}

	r.hoverKey = r.keyAt(r.pointer, pt)
	r.hoverScreen = onScreen(r.pointer, pt)

	for {
		e, ok := r.click.Update(gtx.Source)

		if !ok {
			break
		}

		if e.Kind != gesture.KindClick {
			continue
		}

		p := layout.FPt(e.Position)

		if kr := r.keyAt(p, pt); kr != nil {
			r.press(k, gtx, kr)
			continue
		}

		if onScreen(p, pt) {
			cmd := strings.TrimSpace(string(r.typed))

			if cmd == "" && len(r.lines) == 0 {
				cmd = r.command
			}

			r.typed, r.lines, r.typing, r.shift = nil, nil, false, false
			r.run(cmd)
		}
	}
}

func (r *Computer) pressed(gtx layout.Context, command string) rune {
	if !r.hovered || r.typing {
		return 0
	}

	runes := []rune("$ " + command)
	n := int(gtx.Now.Sub(r.since) / typeStep)

	if n <= 0 || n > len(runes) {
		return 0
	}

	return unicode.ToUpper(runes[n-1])
}

func (r *Computer) details(gtx layout.Context, th *theme.Theme, pt func(vec3) f32.Point, on float32) {
	led := pt(vec3{0.7, 0.2, 0.62})
	rad := float32(gtx.Dp(2.5))
	col := kit.Lerp(th.TextFaint, th.Text, on)
	paint.FillShape(gtx.Ops, col, clip.Ellipse{Min: image.Pt(int(led.X-rad), int(led.Y-rad)), Max: image.Pt(int(led.X+rad), int(led.Y+rad))}.Op(gtx.Ops))

	if on <= 0 {
		return
	}

	lc := kit.Fade(th.Text, 0.05*on)

	for y := screenY0 + 0.03; y < screenY1; y += 0.04 {
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(pt(vec3{screenX0, y, screenZ}))
		p.LineTo(pt(vec3{screenX1, y, screenZ}))
		paint.FillShape(gtx.Ops, lc, clip.Stroke{Path: p.End(), Width: 1}.Op())
	}
}

func onScreen(p f32.Point, pt func(vec3) f32.Point) bool {
	o := pt(vec3{screenX0, screenY0, screenZ})
	ex := pt(vec3{screenX1, screenY0, screenZ}).Sub(o)
	ey := pt(vec3{screenX0, screenY1, screenZ}).Sub(o)
	det := ex.X*ey.Y - ex.Y*ey.X

	if det == 0 {
		return false
	}

	d := p.Sub(o)
	u := (d.X*ey.Y - d.Y*ey.X) / det
	v := (ex.X*d.Y - ex.Y*d.X) / det
	return u >= 0 && u <= 1 && v >= 0 && v <= 1
}
