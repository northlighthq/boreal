package kit

import (
	"image/color"
	"math"
	"time"

	"gioui.org/op"
)

const (
	Fast   = 110 * time.Millisecond
	Normal = 200 * time.Millisecond
	Slow   = 280 * time.Millisecond
)

type Anim struct {
	from, to float32
	start    time.Time
	dur      time.Duration
	init     bool
	used     time.Time
}

func (a *Anim) Set(v float32) {
	a.from, a.to, a.init = v, v, true
	a.start = time.Time{}
}

func (a *Anim) Animate(gtx C, target float32, dur time.Duration) float32 {
	if !a.init {
		a.Set(target)
		return target
	}

	if target != a.to {
		a.from = a.value(gtx.Now)
		a.to = target
		a.start = gtx.Now
		a.dur = dur
	}

	v := a.value(gtx.Now)

	if v != a.to {
		gtx.Execute(op.InvalidateCmd{})
	}

	return v
}

func (a *Anim) Running(now time.Time) bool {
	return a.init && a.value(now) != a.to
}

func (a *Anim) value(now time.Time) float32 {
	if a.start.IsZero() || a.dur <= 0 {
		return a.to
	}

	t := float32(now.Sub(a.start)) / float32(a.dur)

	if t >= 1 {
		return a.to
	}

	if t < 0 {
		t = 0
	}

	return a.from + (a.to-a.from)*EaseOut(t)
}

func EaseOut(t float32) float32 {
	if t >= 1 {
		return 1
	}

	u := 1 - t
	return 1 - u*u*u*u
}

type animKey struct {
	owner any
	kind  uint8
}

const (
	keyPress uint8 = iota + 1
	keyActive
	keyFocus
	keyOpen
	keyPos
	keyWidth
	keyHover uint8 = 255
)

func Key(owner any, kind uint8) animKey {
	return animKey{owner, kind}
}

func (k *Kit) Hover(gtx C, key animKey, on bool) float32 {
	a := k.anim(gtx, key)
	t := float32(0)

	if on {
		t = 1
	}

	return a.Animate(gtx, t, Fast)
}

const animTTL = 30 * time.Second

func (k *Kit) anim(gtx C, key animKey) *Anim {
	if k.anims == nil {
		k.anims = map[animKey]*Anim{}
	}

	k.pruneAnims(gtx.Now)

	a, ok := k.anims[key]

	if !ok {
		a = &Anim{}
		k.anims[key] = a
	}

	a.used = gtx.Now
	return a
}

func (k *Kit) Animated(gtx C, key animKey, target float32, dur time.Duration) float32 {
	return k.anim(gtx, key).Animate(gtx, target, dur)
}

func Lerp(a, b color.NRGBA, t float32) color.NRGBA {
	t = min(max(t, 0), 1)
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(float32(x) + (float32(y)-float32(x))*t))) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: l(a.A, b.A)}
}

func Fade(c color.NRGBA, t float32) color.NRGBA {
	c.A = uint8(float32(c.A) * min(max(t, 0), 1))
	return c
}

func (k *Kit) pruneAnims(now time.Time) {
	if now.Sub(k.pruned) < animTTL {
		return
	}

	k.pruned = now

	for key, a := range k.anims {
		if now.Sub(a.used) > animTTL {
			delete(k.anims, key)
		}
	}
}

func B2F(b bool) float32 {
	if b {
		return 1
	}

	return 0
}
