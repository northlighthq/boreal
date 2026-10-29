package sol

import (
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mascot"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

func ease(x float64) float32 {
	x = min(max(x, 0), 1)
	return float32(1 - math.Pow(1-x, 3))
}

func Sad(gtx layout.Context, th *theme.Theme, size unit.Dp, orbit, t float64) layout.Dimensions {
	px := gtx.Dp(size)
	fpx := float32(px)
	box := fpx * 2
	in := ease(t / 0.9)

	const tau = 0.45
	from := playPose(px, orbit+tau*(1-math.Exp(-t/tau)))
	glide := ease((t - 0.35) / 1.4)
	rest := f32.Pt(box*0.79, box*0.62+fpx*0.03*float32(math.Sin(t*2*math.Pi/3.2)))
	star := from.star.Add(rest.Sub(from.star).Mul(glide))

	sway := float32(0.04 * math.Sin(t*2*math.Pi/5.5))
	center := f32.Pt(box/2, box/2)
	toStar := star.Sub(center)
	look := f32.Pt(toStar.X/fpx, toStar.Y/(fpx*0.38))
	pulse := float32(0.5 + 0.5*math.Sin(t*2*math.Pi/2.6))

	ps := pose{
		star:     star,
		starSize: from.starSize + (fpx*0.1*(0.92+0.08*pulse)-from.starSize)*glide,
		behind:   from.behind && glide < 0.5,
		rock:     from.rock + (sway-from.rock)*in,
		bob:      from.bob*(1-in) - fpx*0.05*in,
		look:     f32.Pt(from.look.X+(look.X*0.6-from.look.X)*in, from.look.Y+(look.Y*0.6-from.look.Y)*in),
	}

	blink := mascot.BlinkAt(orbit + t*0.6)
	st := mascot.State{T: orbit + t*0.3, Blink: blink, Sad: in, Look: ps.look}
	starCol := kit.Lerp(kit.Lerp(th.Text, th.Editor, 0.05), th.TextMuted, glide)
	return scene(gtx, th, px, ps, st, kit.Fade(starCol, 1-glide*(0.45-0.2*pulse)))
}
