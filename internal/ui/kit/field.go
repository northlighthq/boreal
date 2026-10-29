package kit

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const focusRing = unit.Dp(3)

type FieldOpts struct {
	Hint     string
	Icon     *icons.Icon
	Mono     bool
	Invalid  bool
	Trailing W
}

func (k *Kit) Field(gtx C, ed *widget.Editor, o FieldOpts) D {
	th := k.Th
	h := gtx.Dp(th.FieldHeight)
	radius := gtx.Dp(th.Radius)
	focus := k.Animated(gtx, Key(ed, keyFocus), B2F(gtx.Focused(ed)), Normal)

	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h
	width := gtx.Constraints.Max.X

	if gtx.Constraints.Min.X > 0 {
		width = max(gtx.Constraints.Min.X, 0)
	}

	size := image.Pt(width, h)
	r := image.Rectangle{Max: size}

	accent := th.TextFaint

	if o.Invalid {
		accent = th.Danger
		focus = max(focus, 0.8)
	}

	RRect(gtx, r, radius, th.Field)

	gtx.Constraints = layout.Exact(size)
	layout.Inset{Left: 10, Right: 6}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if o.Icon == nil {
					return D{}
				}

				return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D {
					return o.Icon.Layout(gtx, 16, Lerp(th.TextFaint, th.TextMuted, focus))
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.W.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.Y = 0

					if k.Redact != nil && !gtx.Focused(ed) {
						if t := ed.Text(); k.Redact(t) != t {
							dims := k.Editor(gtx, ed, o.Hint, o.Mono)
							Rect(gtx, image.Rectangle{Max: dims.Size}, k.Th.Field)
							k.Label(gtx, TextStyle{Mono: o.Mono}, t)
							return dims
						}
					}

					return k.Editor(gtx, ed, o.Hint, o.Mono)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if o.Trailing == nil {
					return D{}
				}

				return o.Trailing(gtx)
			}),
		)
	})

	Border(gtx, r, radius, 1, Lerp(th.FieldLine, accent, focus))
	return D{Size: size}
}

func (k *Kit) LabeledField(gtx C, label string, ed *widget.Editor, o FieldOpts) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, TextStyle{Size: k.Th.TextSizeSmall, Color: k.Th.TextMuted, Weight: font.Medium}, label)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return k.Field(gtx, ed, o)
		}),
	)
}

func (k *Kit) FormRow(gtx C, label string, labelWidth unit.Dp, w W) D {
	if k.narrow {
		if label == "" {
			return w(gtx)
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, TextStyle{Size: k.Th.TextSizeSmall, Color: k.Th.TextMuted, Weight: font.Medium}, label)
				})
			}),
			layout.Rigid(w),
		)
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(labelWidth)
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			return k.Label(gtx, TextStyle{Color: k.Th.TextMuted}, label)
		}),
		layout.Flexed(1, w),
	)
}

func (k *Kit) Toggle(gtx C, b *widget.Bool) D {
	th := k.Th
	t := k.Animated(gtx, Key(b, keyActive), B2F(b.Value), Normal)
	hv := k.Hover(gtx, Key(b, keyHover), b.Hovered())

	return b.Layout(gtx, func(gtx C) D {
		w, h := gtx.Dp(36), gtx.Dp(20)
		bg := Lerp(Lerp(th.FieldLine, th.TextFaint, hv*0.5), th.Accent, t)
		RRect(gtx, image.Rect(0, 0, w, h), h/2, bg)

		pad := gtx.Dp(3)
		knob := h - 2*pad
		x := pad + int(float32(w-knob-2*pad)*t)
		knobOff := th.Text

		if !th.Dark {
			knobOff = th.Editor
		}

		RRect(gtx, image.Rect(x, pad, x+knob, pad+knob), knob/2, Lerp(knobOff, th.OnAccent, t))
		Cursor(gtx, image.Pt(w, h), pointer.CursorPointer)
		return D{Size: image.Pt(w, h)}
	})
}

func (k *Kit) Checkbox(gtx C, b *widget.Bool, label string) D {
	th := k.Th
	t := k.Animated(gtx, Key(b, keyActive), B2F(b.Value), Fast)

	return b.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				s := gtx.Dp(18)
				r := image.Rect(0, 0, s, s)
				RRect(gtx, r, gtx.Dp(5), Lerp(th.Field, th.Accent, t))
				Border(gtx, r, gtx.Dp(5), 1, Lerp(th.FieldLine, th.Accent, t))

				if t > 0 {
					icons.Check.Layout(gtx, 18, Fade(th.OnAccent, t))
				}

				return D{Size: image.Pt(s, s)}
			}),
			layout.Rigid(Gap(10)),
			layout.Rigid(func(gtx C) D { return k.Text(gtx, label) }),
		)
	})
}

type Segmented struct {
	Value int
	clks  []widget.Clickable
}

func (s *Segmented) Update(gtx C) bool {
	changed := false

	for i := range s.clks {
		if s.clks[i].Clicked(gtx) && s.Value != i {
			s.Value = i
			changed = true
		}
	}

	return changed
}

func (k *Kit) Segmented(gtx C, s *Segmented, options []string) D {
	th := k.Th

	if len(s.clks) != len(options) {
		s.clks = make([]widget.Clickable, len(options))
	}

	s.Update(gtx)

	h := gtx.Dp(th.FieldHeight)
	radius := gtx.Dp(th.Radius)
	pad := gtx.Dp(3)

	xs := make([]int, len(options)+1)
	rec := op.Record(gtx.Ops)

	x := pad
	children := make([]layout.FlexChild, len(options))
	equal := 0

	if k.narrow {
		equal = (gtx.Constraints.Max.X - 2*pad) / len(options)
	}

	for i, opt := range options {
		children[i] = layout.Rigid(func(gtx C) D {
			if equal > 0 {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = equal, equal
			}

			dims := s.clks[i].Layout(gtx, func(gtx C) D {
				selected := s.Value == i
				hv := k.Hover(gtx, Key(&s.clks[i], keyHover), s.clks[i].Hovered())
				on := k.Animated(gtx, Key(&s.clks[i], keyActive), B2F(selected), Normal)
				fg := Lerp(Lerp(th.TextMuted, th.Text, hv), th.Text, on)
				gtx.Constraints.Min.Y = h
				padX := unit.Dp(14)

				if k.narrow {
					padX = 9
				}

				d := layout.Inset{Left: padX, Right: padX}.Layout(gtx, func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, TextStyle{Color: fg, Weight: font.Medium}, opt)
					})
				})
				Cursor(gtx, d.Size, pointer.CursorPointer)
				return d
			})
			xs[i] = x
			x += dims.Size.X
			xs[i+1] = x
			return dims
		})
	}

	gtx.Constraints.Min.X = 0
	off := op.Offset(image.Pt(pad, 0)).Push(gtx.Ops)
	inner := layout.Flex{}.Layout(gtx, children...)
	off.Pop()
	call := rec.Stop()

	size := image.Pt(inner.Size.X+2*pad, inner.Size.Y)
	r := image.Rectangle{Max: size}
	RRect(gtx, r, radius, th.Field)
	Border(gtx, r, radius, 1, th.FieldLine)

	if s.Value >= 0 && s.Value < len(options) {
		px := k.Animated(gtx, Key(s, keyPos), float32(xs[s.Value]), Normal)
		pw := k.Animated(gtx, Key(s, keyWidth), float32(xs[s.Value+1]-xs[s.Value]), Normal)
		pill := image.Rect(int(px), pad, int(px+pw), size.Y-pad)
		RRect(gtx, pill, radius-pad, th.Selection)
		Border(gtx, pill, radius-pad, 1, th.FieldLine)
	}

	call.Add(gtx.Ops)
	return D{Size: size}
}

func (k *Kit) Editor(gtx C, ed *widget.Editor, hint string, mono bool) D {
	st := material.Editor(k.Mat, ed, hint)
	st.Font = Font(font.Normal)

	if mono {
		st.Font = font.Font{Typeface: fonts.Mono}
	}

	st.TextSize = k.Th.TextSize
	st.Color = k.Th.Text
	st.HintColor = k.Th.TextFaint
	st.SelectionColor = theme.WithAlpha(k.Th.Text, 0x30)
	return st.Layout(gtx)
}
