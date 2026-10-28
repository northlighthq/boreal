package sol

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

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mascot"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const orbitPeriod = 2.4

type pose struct {
	star     f32.Point
	starSize float32
	behind   bool
	rock     float32
	bob      float32
	look     f32.Point
}

func playPose(px int, t float64) pose {
	box := float32(px * 2)
	center := f32.Pt(box/2, box/2)
	ang := t * 2 * math.Pi / orbitPeriod
	rx, ry := float64(px)*1.0, float64(px)*0.38
	tilt := -0.35
	ox, oy := rx*math.Cos(ang), ry*math.Sin(ang)
	star := f32.Pt(
		center.X+float32(ox*math.Cos(tilt)-oy*math.Sin(tilt)),
		center.Y+float32(ox*math.Sin(tilt)+oy*math.Cos(tilt)),
	)
	look := star.Sub(center)

	return pose{
		star:     star,
		starSize: float32(px) * 0.17 * float32(1+0.15*math.Sin(t*9)),
		behind:   math.Sin(ang) < 0,
		rock:     float32(0.18 * math.Sin(t*2*math.Pi/orbitPeriod)),
		bob:      float32(math.Abs(math.Sin(t*2*math.Pi/1.2))) * float32(px) * 0.05,
		look:     f32.Pt(look.X/float32(rx), look.Y/float32(ry)),
	}
}

func Playing(gtx layout.Context, th *theme.Theme, size unit.Dp, t float64) layout.Dimensions {
	px := gtx.Dp(size)
	ps := playPose(px, t)
	st := mascot.State{T: t, Blink: mascot.BlinkAt(t), Look: ps.look}
	return scene(gtx, th, px, ps, st, kit.Lerp(th.Text, th.Editor, 0.05))
}

func scene(gtx layout.Context, th *theme.Theme, px int, ps pose, st mascot.State, starCol color.NRGBA) layout.Dimensions {
	box := px * 2
	colors := mascot.DefaultColors

	if !th.Dark {
		colors = mascot.LightColors
	}

	if ps.behind {
		Star(gtx, ps.star, ps.starSize*0.8, kit.Fade(starCol, 0.55))
	}

	o := f32.Pt((float32(box)-float32(px))/2, (float32(box)-float32(px))/2-ps.bob)
	tr := f32.Affine2D{}.Rotate(f32.Pt(float32(px)/2, float32(px)/2), ps.rock).Offset(o)
	s := op.Affine(tr).Push(gtx.Ops)
	mascot.Draw(gtx.Ops, float32(px), st, colors)
	s.Pop()

	if !ps.behind {
		Star(gtx, ps.star, ps.starSize, starCol)
	}

	return layout.Dimensions{Size: image.Pt(box, box)}
}

func Star(gtx layout.Context, c f32.Point, r float32, col color.NRGBA) {
	starShape(gtx, c, r*1.25, color.NRGBA{A: uint8(0x50 * float32(col.A) / 255)})
	starShape(gtx, c, r, col)
}

func starShape(gtx layout.Context, c f32.Point, r float32, col color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(c.X, c.Y-r))
	p.QuadTo(c, f32.Pt(c.X+r, c.Y))
	p.QuadTo(c, f32.Pt(c.X, c.Y+r))
	p.QuadTo(c, f32.Pt(c.X-r, c.Y))
	p.QuadTo(c, f32.Pt(c.X, c.Y-r))
	p.Close()
	paint.FillShape(gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
}
