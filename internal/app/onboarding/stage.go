package onboarding

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mark"
	"github.com/northlighthq/boreal/internal/ui/sol"
)

const (
	jumpTime    = 750 * time.Millisecond
	landTime    = 260 * time.Millisecond
	hopEvery    = 4600 * time.Millisecond
	hopTime     = 420 * time.Millisecond
	burstTime   = 1100 * time.Millisecond
	typeStep    = 24 * time.Millisecond
	contentFade = 380 * time.Millisecond
	starCount   = 24
)

type spot struct {
	center f32.Point
	size   float32
}

type actor struct {
	from    spot
	start   time.Time
	landed  time.Time
	placed  bool
	current spot
}

func (a *actor) jump(from spot, now time.Time) {
	a.from, a.start = from, now
	a.landed = now.Add(jumpTime)
}

func (a *actor) jumping(now time.Time) bool {
	return now.Before(a.landed)
}

type pose struct {
	center f32.Point
	size   float32
	scale  f32.Point
	rot    float32
}

func ease(t float64) float64 {
	return t * t * (3 - 2*t)
}

func clamp01(t float64) float64 {
	return min(max(t, 0), 1)
}

func (a *actor) pose(to spot, now, born time.Time, dp float32) pose {
	p := pose{center: to.center, size: to.size, scale: f32.Pt(1, 1)}
	life := now.Sub(born).Seconds()

	if a.jumping(now) {
		t := clamp01(now.Sub(a.start).Seconds() / jumpTime.Seconds())
		e := float32(ease(t))
		d := to.center.Sub(a.from.center)
		dist := float32(math.Hypot(float64(d.X), float64(d.Y)))
		height := max(90*dp, dist*0.35)
		arc := float32(4*t*(1-t)) * height
		p.center = a.from.center.Add(d.Mul(e)).Sub(f32.Pt(0, arc))
		p.size = a.from.size + (to.size-a.from.size)*e
		stretch := float32(math.Sin(math.Pi * t))
		p.scale = f32.Pt(1-0.08*stretch, 1+0.12*stretch)
		dir := float32(1)

		if d.X < 0 {
			dir = -1
		}

		p.rot = dir * 0.3 * stretch
		return p
	}

	if since := now.Sub(a.landed); !a.landed.IsZero() && since < landTime {
		k := since.Seconds() / landTime.Seconds()
		squash := float32(math.Sin(math.Pi*k) * (1 - k))
		p.scale = f32.Pt(1+0.16*squash, 1-0.22*squash)
		return p
	}

	p.center.Y += float32(math.Sin(life*2*math.Pi/2.6)) * 5 * dp
	p.rot = float32(math.Sin(life*2*math.Pi/3.4)) * 0.05

	if !a.landed.IsZero() {
		cycle := now.Sub(a.landed) % hopEvery
		start := hopEvery - hopTime

		if cycle > start {
			k := (cycle - start).Seconds() / hopTime.Seconds()
			p.center.Y -= float32(math.Sin(math.Pi*k)) * 20 * dp
			squash := float32(math.Sin(math.Pi*k*2)) * 0.05
			p.scale = f32.Pt(1-squash, 1+squash)
		}
	}

	return p
}

func drawSol(gtx layout.Context, k *kit.Kit, s *sol.Sol, p pose) {
	size := p.size
	topLeft := p.center.Sub(f32.Pt(size/2, size/2))
	tr := f32.Affine2D{}.
		Scale(f32.Pt(size/2, size), p.scale).
		Rotate(f32.Pt(size/2, size/2), p.rot).
		Offset(topLeft)
	defer op.Affine(tr).Push(gtx.Ops).Pop()

	g := gtx
	g.Constraints = layout.Exact(image.Pt(int(size), int(size)))
	defer op.Offset(image.Pt(int(size*0.19), int(size*0.19))).Push(gtx.Ops).Pop()
	mark.Layout(g, unit.Dp(size*0.62/gtx.Metric.PxPerDp))
}

type burst struct {
	at    f32.Point
	born  time.Time
	count int
	reach float32
}

func (b burst) draw(gtx layout.Context, now time.Time, col color.NRGBA) bool {
	t := now.Sub(b.born).Seconds() / burstTime.Seconds()

	if t >= 1 {
		return false
	}

	out := float32(1 - math.Pow(1-t, 3))

	for i := range b.count {
		h := hash(uint32(i)*7919 + uint32(b.born.UnixNano()))
		ang := float64(i)/float64(b.count)*2*math.Pi + (h-0.5)*0.6
		dist := b.reach * (0.6 + 0.4*float32(hash(uint32(i)*104729)))
		pos := b.at.Add(f32.Pt(float32(math.Cos(ang))*dist*out, float32(math.Sin(ang))*dist*out+float32(t*t)*b.reach*0.4))
		r := float32(gtx.Dp(7)) * (1 - float32(t)) * float32(0.6+0.6*h)
		sol.Star(gtx, pos, r, kit.Fade(col, float32(1-t)))
	}

	return true
}

func hash(n uint32) float64 {
	n ^= n >> 16
	n *= 0x7feb352d
	n ^= n >> 15
	n *= 0x846ca68b
	n ^= n >> 16
	return float64(n) / float64(math.MaxUint32)
}

func stars(gtx layout.Context, size image.Point, t float64, col color.NRGBA, avoid image.Rectangle) {
	for i := range starCount {
		x0 := hash(uint32(i) * 3)
		y0 := hash(uint32(i)*3 + 1)
		speed := 0.006 + 0.014*hash(uint32(i)*3+2)
		phase := hash(uint32(i)*7+5) * 2 * math.Pi
		y := math.Mod(y0-t*speed+10, 1)
		x := x0 + 0.01*math.Sin(t*0.4+phase)
		twinkle := 0.5 + 0.5*math.Sin(t*(0.8+hash(uint32(i)*11))*2+phase)
		r := float32(gtx.Dp(2)) + float32(gtx.Dp(4))*float32(hash(uint32(i)*13))
		pos := f32.Pt(float32(x)*float32(size.X), float32(y)*float32(size.Y))
		fade := float32(0.12 + 0.3*twinkle)

		if image.Pt(int(pos.X), int(pos.Y)).In(avoid.Inset(-gtx.Dp(12))) {
			fade *= 0.15
		}

		sol.Star(gtx, pos, r, kit.Fade(col, fade))
	}
}

func bubble(gtx layout.Context, k *kit.Kit, text string, shown int, at f32.Point, width int, alpha float32) {
	th := k.Th
	pad := gtx.Dp(14)
	tail := float32(gtx.Dp(9))

	measure := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(width-2*pad, gtx.Dp(400))}
	full := k.Label(g, kit.TextStyle{MaxLines: 5}, text)
	measure.Stop()

	w, h := full.Size.X+2*pad, full.Size.Y+2*pad
	x := int(at.X) - w/2
	y := int(at.Y + tail)

	if alpha < 1 {
		defer paint.PushOpacity(gtx.Ops, max(alpha, 0.01)).Pop()
	}
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()

	r := image.Rect(0, 0, w, h)
	radius := gtx.Dp(14)
	kit.RRect(gtx, r, radius, th.Panel)
	kit.Border(gtx, r, radius, 1, th.Border)

	var p clip.Path
	cx := float32(w / 2)
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(cx-tail, 1))
	p.LineTo(f32.Pt(cx, -tail+1))
	p.LineTo(f32.Pt(cx+tail, 1))
	p.Close()
	paint.FillShape(gtx.Ops, th.Panel, clip.Outline{Path: p.End()}.Op())

	if shown <= 0 {
		return
	}

	runes := []rune(text)
	shown = min(shown, len(runes))
	defer op.Offset(image.Pt(pad, pad)).Push(gtx.Ops).Pop()
	k.Label(g, kit.TextStyle{MaxLines: 5}, string(runes[:shown]))
}

func typed(text string, since time.Duration) int {
	if since < 0 {
		return 0
	}

	return min(int(since/typeStep), len([]rune(text)))
}
