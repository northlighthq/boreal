package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
)

type SheetItem struct {
	Icon   *icons.Icon
	Label  string
	Value  string
	Danger bool
	Do     func()
}

type Sheet struct {
	Title    string
	Subtitle string
	Items    []SheetItem
	clks     []widget.Clickable
	appear   kit.Anim
	closing  bool
	chosen   func()
}

func (s *Sheet) Dismiss() {
	s.closing = true
}

func (s *Sheet) Layout(gtx C, k *kit.Kit) bool {
	th := k.Th

	if len(s.clks) != len(s.Items) {
		s.clks = make([]widget.Clickable, len(s.Items))
	}

	for i := range s.clks {
		if s.clks[i].Clicked(gtx) && !s.closing {
			s.chosen = s.Items[i].Do
			s.closing = true
		}
	}

	target := float32(1)

	if s.closing {
		target = 0
	}

	p := s.appear.Animate(gtx, target, kit.Normal)

	if s.closing && p == 0 {
		if s.chosen != nil {
			s.chosen()
		}

		return true
	}

	size := gtx.Constraints.Max
	scrim(gtx, k, size, p, func() { s.closing = true })

	rec := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(size.X, 0), Max: size}
	dims := layout.Inset{Top: 10, Bottom: 18}.Layout(g, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return layout.N.Layout(gtx, func(gtx C) D {
					r := image.Rect(0, 0, gtx.Dp(36), gtx.Dp(4))
					kit.RRect(gtx, r, r.Max.Y/2, th.Border)
					return D{Size: r.Max}
				})
			}),
		}

		if s.Title != "" {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: 24, Right: 24, Top: 18, Bottom: 8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: TextBody + 1, Weight: font.SemiBold, MaxLines: 1}, s.Title)
						}),
						layout.Rigid(func(gtx C) D {
							if s.Subtitle == "" {
								return D{}
							}

							return layout.Inset{Top: 2}.Layout(gtx, func(gtx C) D {
								return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, MaxLines: 1}, s.Subtitle)
							})
						}),
					)
				})
			}))
		}

		for i, it := range s.Items {
			children = append(children, layout.Rigid(func(gtx C) D {
				fg := th.Text

				if it.Danger {
					fg = th.Danger
				}

				return Row(gtx, k, &s.clks[i], RowOpts{
					Title:  it.Label,
					Value:  it.Value,
					Danger: it.Danger,
					Lead: func(gtx C) D {
						return layout.Inset{Left: 8}.Layout(gtx, func(gtx C) D { return it.Icon.Layout(gtx, 22, fg) })
					},
				})
			}))
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := rec.Stop()

	y := size.Y - int(float32(dims.Size.Y)*easeOut(p))
	defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
	radius := gtx.Dp(22)
	r := image.Rectangle{Max: image.Pt(size.X, dims.Size.Y+radius)}
	paint.FillShape(gtx.Ops, th.Popup, clip.RRect{Rect: r, NE: radius, NW: radius}.Op(gtx.Ops))
	block(gtx, s, r.Max)
	call.Add(gtx.Ops)
	return false
}

type Dialog struct {
	Title    string
	Message  string
	Detail   string
	Field    *widget.Editor
	Hint     string
	Confirm  string
	Cancel   string
	Danger   bool
	Reply    func(ok bool, text string)
	ok       widget.Clickable
	no       widget.Clickable
	appear   kit.Anim
	closing  bool
	answered bool
	result   bool
	focused  bool
}

func (d *Dialog) Dismiss() {
	d.closing = true
}

func (d *Dialog) Layout(gtx C, k *kit.Kit) bool {
	th := k.Th

	if d.ok.Clicked(gtx) {
		d.result, d.closing = true, true
	}

	if d.no.Clicked(gtx) {
		d.closing = true
	}

	if d.Field != nil && !d.focused {
		d.focused = true
		gtx.Execute(key.FocusCmd{Tag: d.Field})
	}

	if d.Field != nil {
		for {
			ev, ok := d.Field.Update(gtx)

			if !ok {
				break
			}

			if _, ok := ev.(widget.SubmitEvent); ok {
				d.result, d.closing = true, true
			}
		}
	}

	target := float32(1)

	if d.closing {
		target = 0
	}

	p := d.appear.Animate(gtx, target, kit.Normal)

	if d.closing && !d.answered {
		d.answered = true
		text := ""

		if d.Field != nil {
			text = d.Field.Text()
		}

		if d.Reply != nil {
			d.Reply(d.result, text)
		}
	}

	if d.closing && p == 0 {
		return true
	}

	size := gtx.Constraints.Max
	scrim(gtx, k, size, p, nil)
	defer paint.PushOpacity(gtx.Ops, p).Pop()

	w := min(size.X-gtx.Dp(48), gtx.Dp(400))
	rec := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, size.Y)}
	dims := layout.UniformInset(24).Layout(g, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: TextBar, Weight: font.SemiBold, MaxLines: 2}, d.Title)
			}),
		}

		if d.Message != "" {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: TextSub, Color: th.TextMuted, MaxLines: 8}, d.Message)
				})
			}))
		}

		if d.Detail != "" {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 14}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					rec := op.Record(gtx.Ops)
					dims := layout.UniformInset(12).Layout(gtx, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: TextCaption, Mono: true, MaxLines: 4}, d.Detail)
					})
					call := rec.Stop()
					kit.RRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(10), th.Field)
					call.Add(gtx.Ops)
					return dims
				})
			}))
		}

		if d.Field != nil {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 16}.Layout(gtx, func(gtx C) D {
					return Field(gtx, k, d.Field, FieldOpts{Hint: d.Hint})
				})
			}))
		}

		children = append(children,
			layout.Rigid(Gap(22)),
			layout.Rigid(func(gtx C) D {
				kind := Primary

				if d.Danger {
					kind = Destructive
				}

				return Button(gtx, k, &d.ok, kind, nil, d.Confirm, true)
			}),
		)

		if d.Cancel != "" {
			children = append(children,
				layout.Rigid(Gap(10)),
				layout.Rigid(func(gtx C) D { return Button(gtx, k, &d.no, Secondary, nil, d.Cancel, true) }),
			)
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := rec.Stop()

	off := image.Pt((size.X-dims.Size.X)/2, (size.Y-dims.Size.Y)/2+int((1-easeOut(p))*float32(gtx.Dp(16))))
	defer op.Offset(off).Push(gtx.Ops).Pop()
	k.SurfaceRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(22), th.Popup, true)
	block(gtx, d, dims.Size)
	call.Add(gtx.Ops)
	return false
}

func scrim(gtx C, k *kit.Kit, size image.Point, p float32, tap func()) {
	tag := &k.Th

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Release})

		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Release && tap != nil {
			tap()
		}
	}

	r := image.Rectangle{Max: size}
	kit.Rect(gtx, r, kit.Fade(k.Th.Scrim, p))
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
}

func block(gtx C, tag event.Tag, size image.Point) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Release}); !ok {
			break
		}
	}

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
}

func easeOut(p float32) float32 {
	return 1 - (1-p)*(1-p)*(1-p)
}
