package kit

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/ui/shape"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

type (
	C = layout.Context
	D = layout.Dimensions
	W = layout.Widget
)

type Kit struct {
	Th     *theme.Theme
	Shaper *text.Shaper
	Mat    *material.Theme
	Exit   float32

	anims  map[animKey]*Anim
	pruned time.Time

	pointer f32.Point
	window  image.Point

	narrow  bool
	presses int
	active  time.Time

	Redact func(string) string
}

func New(th *theme.Theme, shaper *text.Shaper) *Kit {
	k := &Kit{Shaper: shaper}
	k.SetTheme(th)
	return k
}

func (k *Kit) SetTheme(th *theme.Theme) {
	k.Th = th

	m := material.NewTheme()
	m.Shaper = k.Shaper
	m.Face = fonts.UI
	m.TextSize = th.TextSize
	m.Palette = material.Palette{Fg: th.Text, Bg: th.Panel, ContrastBg: th.Accent, ContrastFg: th.OnAccent}
	k.Mat = m
}

func Font(w font.Weight) font.Font {
	return font.Font{Typeface: fonts.UI, Weight: w}
}

type TextStyle struct {
	Size     unit.Sp
	Color    color.NRGBA
	Weight   font.Weight
	MaxLines int
	Align    text.Alignment
	Mono     bool
}

func (k *Kit) Label(gtx C, st TextStyle, s string) D {
	if k.Redact != nil {
		s = k.Redact(s)
	}

	if st.Size == 0 {
		st.Size = k.Th.TextSize
	}

	if st.Color == (color.NRGBA{}) {
		st.Color = k.Th.Text
	}

	if st.MaxLines == 0 {
		st.MaxLines = 1
	}

	f := Font(st.Weight)

	if st.Mono {
		f = font.Font{Typeface: fonts.Mono, Weight: st.Weight}
	}

	minX := gtx.Constraints.Min.X
	gtx.Constraints.Min.Y = 0

	if st.Align == text.Start {
		gtx.Constraints.Min.X = 0
	}

	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: st.Color}.Add(gtx.Ops)
	call := mat.Stop()

	dims := widget.Label{MaxLines: st.MaxLines, Alignment: st.Align}.Layout(gtx, k.Shaper, f, st.Size, s, call)
	dims.Size.X = max(dims.Size.X, minX)
	return dims
}

func (k *Kit) Text(gtx C, s string) D {
	return k.Label(gtx, TextStyle{}, s)
}

func (k *Kit) Muted(gtx C, s string) D {
	return k.Label(gtx, TextStyle{Color: k.Th.TextMuted}, s)
}

func (k *Kit) Small(gtx C, s string) D {
	return k.Label(gtx, TextStyle{Size: k.Th.TextSizeSmall, Color: k.Th.TextMuted}, s)
}

func (k *Kit) Strong(gtx C, s string) D {
	return k.Label(gtx, TextStyle{Weight: font.Medium}, s)
}

func (k *Kit) Title(gtx C, s string) D {
	return k.Label(gtx, TextStyle{Size: k.Th.TextSizeTitle, Weight: font.Medium}, s)
}

func Fill(gtx C, c color.NRGBA) D {
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Min}.Op())
	return D{Size: gtx.Constraints.Min}
}

func Rect(gtx C, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

func RRect(gtx C, r image.Rectangle, radius int, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

func Border(gtx C, r image.Rectangle, radius int, width float32, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: shape.Ring(gtx.Ops, r, float32(radius), width)}.Op())
}

func Background(gtx C, bg color.NRGBA, radius unit.Dp, w W) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()

	if bg.A > 0 {
		RRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(radius), bg)
	}

	call.Add(gtx.Ops)
	return dims
}

func (k *Kit) Divider(gtx C, vertical bool) D {
	size := image.Pt(gtx.Constraints.Max.X, 1)

	if vertical {
		size = image.Pt(1, gtx.Constraints.Max.Y)
	}

	Rect(gtx, image.Rectangle{Max: size}, k.Th.Divider)
	return D{Size: size}
}

func (k *Kit) Separator(gtx C, vertical bool) D {
	size := image.Pt(gtx.Constraints.Max.X, 1)

	if vertical {
		size = image.Pt(1, gtx.Constraints.Max.Y)
	}

	Rect(gtx, image.Rectangle{Max: size}, k.Th.Border)
	return D{Size: size}
}

func Cursor(gtx C, size image.Point, c pointer.Cursor) {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	c.Add(gtx.Ops)
}

func Gap(dp unit.Dp) W {
	return func(gtx C) D {
		px := gtx.Dp(dp)
		return D{Size: image.Pt(px, px)}
	}
}

func PadXY(x, y unit.Dp, w W) W {
	return func(gtx C) D {
		return layout.Inset{Left: x, Right: x, Top: y, Bottom: y}.Layout(gtx, w)
	}
}

func (k *Kit) Avatar(gtx C, initials string, bg color.NRGBA, size unit.Dp) D {
	px := gtx.Dp(size)
	r := image.Rect(0, 0, px, px)
	RRect(gtx, r, px/4, k.Th.Selection)
	Border(gtx, r, px/4, 1, k.Th.FieldLine)

	gtx.Constraints = layout.Exact(image.Pt(px, px))
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		return k.Label(gtx, TextStyle{
			Size:   unit.Sp(float32(size) * 0.42),
			Color:  k.Th.TextMuted,
			Weight: font.Medium,
		}, initials)
	})
}

func Dot(gtx C, c color.NRGBA, size unit.Dp) D {
	px := gtx.Dp(size)
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(px, px)}.Op(gtx.Ops))
	return D{Size: image.Pt(px, px)}
}

func (k *Kit) Badge(gtx C, s string, c color.NRGBA) D {
	bg := theme.WithAlpha(c, 0x2e)
	return Background(gtx, bg, k.Th.RadiusSmall, PadXY(6, 2, func(gtx C) D {
		return k.Label(gtx, TextStyle{Size: k.Th.TextSizeSmall - 1, Color: c, Weight: font.Medium}, s)
	}))
}

func (k *Kit) Kbd(gtx C, s string) D {
	return k.Label(gtx, TextStyle{Size: k.Th.TextSizeSmall, Color: k.Th.TextFaint}, s)
}

func (k *Kit) SectionTitle(gtx C, s string) D {
	if s != "" {
		s = strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}

	return k.Label(gtx, TextStyle{Size: k.Th.TextSizeSmall, Color: k.Th.TextMuted, Weight: font.SemiBold}, s)
}

func (k *Kit) ReadPointer(gtx C) {
	k.window = gtx.Constraints.Max

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &k.pointer, Kinds: pointer.Move | pointer.Press | pointer.Drag | pointer.Enter})

		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok {
			k.pointer = e.Position
			k.active = time.Now()

			if e.Kind == pointer.Press {
				k.presses++
			}
		}
	}
}

func (k *Kit) TrackPointer(gtx C) {
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: k.window}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &k.pointer)
}

func (k *Kit) Pointer() f32.Point { return k.pointer }

func (k *Kit) Presses() int { return k.presses }

func (k *Kit) CodeBox(gtx C, text string, c color.NRGBA) D {
	th := k.Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Left: 10, Right: 10, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return k.Label(gtx, TextStyle{Mono: true, Size: th.TextSizeSmall, Color: c, MaxLines: 8}, text)
	})
	call := rec.Stop()
	r := image.Rectangle{Max: dims.Size}
	RRect(gtx, r, gtx.Dp(th.RadiusSmall), Lerp(th.Field, th.Window, 0.5))
	Border(gtx, r, gtx.Dp(th.RadiusSmall), 1, th.Border)
	call.Add(gtx.Ops)
	return dims
}

func (k *Kit) LastActive() time.Time { return k.active }
