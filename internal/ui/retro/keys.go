package retro

import (
	"image"
	"image/color"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	kbX0, kbX1 = -0.88, 0.88
	kbZ0, kbZ1 = 0.74, 1.58
	kbTop      = 0.09

	keyFlash = 160 * time.Millisecond
)

const (
	keyChar = iota
	keyBack
	keyEnter
	keyShift
	keySpace
	keyNone
)

type vkey struct {
	label   string
	ch, alt rune
	w       float64
	action  int
}

func chars(s, shifted string) []vkey {
	sh := []rune(shifted)
	var out []vkey

	for i, r := range []rune(s) {
		out = append(out, vkey{label: strings.ToUpper(string(r)), ch: r, alt: sh[i], w: 1})
	}

	return out
}

func wide(label string, w float64, action int) vkey {
	return vkey{label: label, w: w, action: action}
}

var keyboardRows = [][]vkey{
	append(chars("1234567890-=", "!@#$%^&*()_+"), wide("bksp", 2, keyBack)),
	append(append([]vkey{wide("tab", 1.5, keyNone)}, chars("qwertyuiop[]", "QWERTYUIOP{}")...), vkey{label: "\\", ch: '\\', alt: '|', w: 1.5}),
	append(append([]vkey{wide("caps", 1.75, keyNone)}, chars("asdfghjkl;'", "ASDFGHJKL:\"")...), wide("enter", 2.25, keyEnter)),
	append(append([]vkey{wide("shift", 2.25, keyShift)}, chars("zxcvbnm,./", "ZXCVBNM<>?")...), wide("shift", 2.75, keyShift)),
	{wide("ctrl", 1.25, keyNone), wide("win", 1.25, keyNone), wide("alt", 1.25, keyNone), wide("", 6.25, keySpace),
		wide("alt", 1.25, keyNone), wide("fn", 1.25, keyNone), wide("ctrl", 1.5, keyNone)},
}

type keyRect struct {
	k              *vkey
	x0, z0, x1, z1 float64
}

func keyLayout() []keyRect {
	const (
		x0, x1 = kbX0 + 0.05, kbX1 - 0.05
		z0, z1 = kbZ0 + 0.05, kbZ1 - 0.05
		gap    = 0.014
	)
	rowH := (z1 - z0) / float64(len(keyboardRows))
	var out []keyRect

	for ri := range keyboardRows {
		row := keyboardRows[ri]
		units := 0.0

		for _, k := range row {
			units += k.w
		}

		unit := (x1 - x0 - gap*float64(len(row)-1)) / units
		x := x0
		az := z0 + float64(ri)*rowH + gap/2

		for i := range row {
			w := row[i].w * unit
			out = append(out, keyRect{k: &row[i], x0: x, z0: az, x1: x + w, z1: az + rowH - gap})
			x += w + gap
		}
	}

	return out
}

func (r *Computer) keyAt(p f32.Point, pt func(vec3) f32.Point) *keyRect {
	y := keyTop(false)
	o := pt(vec3{0, y, 0})
	ex := pt(vec3{1, y, 0}).Sub(o)
	ez := pt(vec3{0, y, 1}).Sub(o)
	det := ex.X*ez.Y - ex.Y*ez.X

	if det == 0 {
		return nil
	}

	d := p.Sub(o)
	x := float64((d.X*ez.Y - d.Y*ez.X) / det)
	z := float64((ex.X*d.Y - ex.Y*d.X) / det)

	for i := range r.keyRects {
		kr := &r.keyRects[i]

		if x >= kr.x0 && x <= kr.x1 && z >= kr.z0 && z <= kr.z1 {
			return kr
		}
	}

	return nil
}

func (r *Computer) press(k *kit.Kit, gtx layout.Context, kr *keyRect) {
	r.flashKey, r.flashAt = kr.k, gtx.Now
	r.typing = true

	switch kr.k.action {
	case keyChar:
		ch := kr.k.ch

		if r.shift {
			ch = kr.k.alt
			r.shift = false
		}

		if len(r.typed) < 120 {
			r.typed = append(r.typed, ch)
		}
	case keySpace:
		r.typed = append(r.typed, ' ')
	case keyBack:
		if len(r.typed) > 0 {
			r.typed = r.typed[:len(r.typed)-1]
		}
	case keyShift:
		r.shift = !r.shift
	case keyEnter:
		r.lines = append(r.lines, "$ "+string(r.typed))

		if len(r.lines) > 3 {
			r.lines = r.lines[len(r.lines)-3:]
		}

		r.typed, r.shift = nil, false
	}

	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(keyFlash)})
}

const (
	keyCapH     = 0.05
	keyCapInset = 0.013
)

func keyTop(down bool) float64 {
	if down {
		return kbTop + keyCapH*0.3
	}

	return kbTop + keyCapH
}

func (r *Computer) keys(k *kit.Kit, gtx layout.Context, pt func(vec3) f32.Point, on float32, down rune) {
	th := k.Th

	line := kit.Lerp(th.TextFaint, th.TextMuted, 0.45+0.4*on)
	width := float32(gtx.Dp(1))
	flash := r.flashKey != nil && gtx.Now.Sub(r.flashAt) < keyFlash

	topFill := kit.Lerp(th.Editor, th.Text, 0.06)
	frontFill := kit.Lerp(th.Editor, th.Text, 0.02)
	sideFill := kit.Lerp(th.Editor, th.Window, 0.6)

	poly := func(ps ...vec3) clip.PathSpec {
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(pt(ps[0]))

		for _, q := range ps[1:] {
			p.LineTo(pt(q))
		}

		p.Close()
		return p.End()
	}
	face := func(fill color.NRGBA, ps ...vec3) {
		paint.FillShape(gtx.Ops, fill, clip.Outline{Path: poly(ps...)}.Op())
		paint.FillShape(gtx.Ops, line, clip.Stroke{Path: poly(ps...), Width: width}.Op())
	}

	for i := range r.keyRects {
		kr := &r.keyRects[i]
		lit := flash && kr.k == r.flashKey ||
			down != 0 && (kr.k.action == keyChar && unicode.ToUpper(kr.k.ch) == down || kr.k.action == keySpace && down == ' ') ||
			r.shift && kr.k.action == keyShift
		y0, y1 := kbTop, keyTop(lit)
		d := keyCapInset
		b0, b1 := vec3{kr.x0, y0, kr.z0}, vec3{kr.x1, y0, kr.z1}
		t0, t1 := vec3{kr.x0 + d, y1, kr.z0 + d}, vec3{kr.x1 - d, y1, kr.z1 - d}

		face(frontFill, vec3{b0.x, y0, b1.z}, vec3{b1.x, y0, b1.z}, vec3{t1.x, y1, t1.z}, vec3{t0.x, y1, t1.z})
		face(sideFill, vec3{b1.x, y0, b1.z}, vec3{b1.x, y0, b0.z}, vec3{t1.x, y1, t0.z}, vec3{t1.x, y1, t1.z})
		top := topFill

		switch {
		case lit:
			top = kit.Lerp(topFill, th.Text, 0.4)
		case kr == r.hoverKey:
			top = kit.Lerp(topFill, th.Text, 0.14)
		}

		face(top, vec3{t0.x, y1, t0.z}, vec3{t1.x, y1, t0.z}, vec3{t1.x, y1, t1.z}, vec3{t0.x, y1, t1.z})
		r.keyLabel(k, gtx, pt, kr, y1, on)
	}
}

func (r *Computer) keyLabel(k *kit.Kit, gtx layout.Context, pt func(vec3) f32.Point, kr *keyRect, y float64, on float32) {
	if kr.k.label == "" {
		return
	}

	th := k.Th
	origin := pt(vec3{0, y, 0})
	ex := pt(vec3{0.01, y, 0}).Sub(origin)
	ey := pt(vec3{0, y, 0.01}).Sub(origin)
	tr := f32.NewAffine2D(ex.X, ey.X, origin.X, ex.Y, ey.Y, origin.Y)

	g := gtx
	g.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
	size := unit.Sp(6)

	if kr.k.action != keyChar {
		size = 4.2
	}

	d := keyCapInset
	w := int((kr.x1 - kr.x0 - 2*d) * 100)
	h := int((kr.z1 - kr.z0 - 2*d) * 100)
	st := op.Affine(tr).Push(gtx.Ops)
	off := op.Offset(image.Pt(int((kr.x0+d)*100), int((kr.z0+d)*100))).Push(gtx.Ops)
	g.Constraints = layout.Exact(image.Pt(w, h))
	color := kit.Lerp(th.TextFaint, th.Text, 0.35+0.45*on)
	layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions {
		return k.Label(gtx, kit.TextStyle{Size: size, Weight: font.Medium, Color: color}, kr.k.label)
	})
	off.Pop()
	st.Pop()
}
