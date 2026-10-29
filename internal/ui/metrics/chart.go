package metrics

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const historyLen = 60

type History struct {
	CPU, Mem, Disk []float64
	Rx, Tx         []float64
}

func (h *History) Push(s sshx.Stats) {
	push := func(xs []float64, v float64) []float64 {
		xs = append(xs, v)

		if len(xs) > historyLen {
			xs = xs[len(xs)-historyLen:]
		}

		return xs
	}

	if s.CPU >= 0 {
		h.CPU = push(h.CPU, s.CPU)
	}

	h.Mem = push(h.Mem, ratio(s.MemUsed, s.MemTotal))
	h.Disk = push(h.Disk, ratio(s.DiskUsed, s.DiskTotal))

	if s.RxRate >= 0 {
		h.Rx = push(h.Rx, s.RxRate)
		h.Tx = push(h.Tx, s.TxRate)
	}
}

func (h History) Clone() History {
	cp := func(xs []float64) []float64 { return append([]float64(nil), xs...) }
	return History{CPU: cp(h.CPU), Mem: cp(h.Mem), Disk: cp(h.Disk), Rx: cp(h.Rx), Tx: cp(h.Tx)}
}

func level(th *theme.Theme, frac float64) color.NRGBA {
	switch {
	case frac > 0.85:
		return th.Danger
	case !th.Colorful():
		return th.TextMuted
	case frac > 0.6:
		return th.Warning
	}

	return th.Success
}

func peak(series ...[]float64) float64 {
	m := 0.0

	for _, xs := range series {
		for _, v := range xs {
			m = max(m, v)
		}
	}

	return m
}

func points(series []float64, top float64, size image.Point) []f32.Point {
	n := historyLen

	if len(series) < 2 || top <= 0 {
		return nil
	}

	step := float32(size.X) / float32(n-1)
	off := float32(n-len(series)) * step
	out := make([]f32.Point, len(series))

	for i, v := range series {
		f := float32(min(max(v/top, 0), 1))
		out[i] = f32.Pt(off+float32(i)*step, float32(size.Y)*(1-f))
	}

	return out
}

func area(gtx C, pts []f32.Point, h int, fill color.NRGBA) {
	if len(pts) < 2 {
		return
	}

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(pts[0].X, float32(h)))

	for _, pt := range pts {
		p.LineTo(pt)
	}

	p.LineTo(f32.Pt(pts[len(pts)-1].X, float32(h)))
	p.Close()
	paint.FillShape(gtx.Ops, fill, clip.Outline{Path: p.End()}.Op())
}

func line(gtx C, pts []f32.Point, width float32, col color.NRGBA) {
	if len(pts) < 2 {
		return
	}

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(pts[0].X, pts[0].Y-width/2))

	for _, pt := range pts[1:] {
		p.LineTo(f32.Pt(pt.X, pt.Y-width/2))
	}

	for i := len(pts) - 1; i >= 0; i-- {
		p.LineTo(f32.Pt(pts[i].X, pts[i].Y+width/2))
	}

	p.Close()
	paint.FillShape(gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
}

type series struct {
	values []float64
	color  color.NRGBA
	fill   bool
}

func chart(gtx C, size image.Point, top float64, ss ...series) D {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pad := gtx.Dp(1)
	inner := image.Pt(size.X, size.Y-2*pad)
	defer op.Offset(image.Pt(0, pad)).Push(gtx.Ops).Pop()

	for _, s := range ss {
		pts := points(s.values, top, inner)

		if s.fill {
			area(gtx, pts, inner.Y, kit.Fade(s.color, 0.22))
		}

		line(gtx, pts, float32(gtx.Dp(1))*1.5, s.color)
	}

	return D{Size: size}
}

func netColor(th *theme.Theme) color.NRGBA {
	if th.Colorful() {
		return th.Accent
	}

	return th.Text
}

func iconColor(th *theme.Theme, m metric) color.NRGBA {
	if !th.Colorful() || len(m.chart) == 0 {
		return th.TextMuted
	}

	return m.chart[0].color
}
