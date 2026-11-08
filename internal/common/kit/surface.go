package kit

import (
	"image"
	"image/color"

	"gioui.org/op"
	"gioui.org/unit"
)

type SurfaceOpts struct {
	Radius unit.Dp
	Fill   color.NRGBA
	Shadow bool
}

func (k *Kit) Surface(gtx C, o SurfaceOpts, w W) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()

	k.SurfaceRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(o.Radius), o.Fill, o.Shadow)
	call.Add(gtx.Ops)
	return dims
}

func (k *Kit) SurfaceRect(gtx C, r image.Rectangle, radius int, fill color.NRGBA, shadow bool) {
	if shadow {
		off := op.Offset(r.Min).Push(gtx.Ops)
		k.shadow(gtx, r.Size(), radius)
		off.Pop()
	}

	if fill.A > 0 {
		RRect(gtx, r, radius, fill)
	}

	Border(gtx, r, radius, 1, k.Th.Border)
}
