package kit

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/ui/icons"
)

const (
	menuWidth    = unit.Dp(220)
	menuItemH    = unit.Dp(28)
	catcherReach = 1 << 14
)

type MenuItem struct {
	Label    string
	Icon     *icons.Icon
	Shortcut string
	Danger   bool
	Disabled bool
}

func Separator() MenuItem {
	return MenuItem{}
}

type Menu struct {
	Items []MenuItem
	Width unit.Dp

	open       bool
	pos        image.Point
	clks       []widget.Clickable
	chosen     int
	picked     bool
	appear     Anim
	focused    bool
	focusTries int
	active     int
	origin     image.Point
	known      bool
	size       image.Point
	card       bool
}

func (m *Menu) Open(items []MenuItem, pos image.Point) {
	m.Items = items
	m.pos = pos
	m.open = true
	m.picked = false
	m.appear.Set(0)
	m.active = -1
	m.known = false
	m.focused = false
	m.focusTries = 0
	m.clks = make([]widget.Clickable, len(items))
}

func (m *Menu) IsOpen() bool {
	return m.open
}

func (m *Menu) Close() {
	m.open = false
}

func (m *Menu) Chosen() (int, bool) {
	if !m.picked {
		return 0, false
	}

	m.picked = false
	return m.chosen, true
}

func (k *Kit) Menu(gtx C, m *Menu) {
	if !m.open {
		return
	}

	for i := range m.clks {
		if m.clks[i].Clicked(gtx) && !m.Items[i].Disabled && m.Items[i].Label != "" {
			m.chosen = i
			m.picked = true
			m.open = false
			return
		}
	}

	for {
		ev, ok := gtx.Event(
			pointer.Filter{Target: m, Kinds: pointer.Press | pointer.Move | pointer.Enter},
			key.FocusFilter{Target: m},
			key.Filter{Focus: m, Name: key.NameEscape},
			key.Filter{Focus: m, Name: key.NameUpArrow},
			key.Filter{Focus: m, Name: key.NameDownArrow},
			key.Filter{Focus: m, Name: key.NameReturn},
			key.Filter{Focus: m, Name: key.NameEnter},
		)

		if !ok {
			break
		}

		switch e := ev.(type) {
		case pointer.Event:
			if e.Kind == pointer.Press {
				m.open = false
				return
			}

			if !m.known {
				m.origin = k.pointer.Sub(e.Position).Round()
				m.known = true
			}
		case key.Event:
			if e.State != key.Press {
				continue
			}

			switch e.Name {
			case key.NameUpArrow:
				m.step(-1)
			case key.NameDownArrow:
				m.step(1)
			case key.NameReturn, key.NameEnter:
				if m.active >= 0 {
					m.chosen, m.picked, m.open = m.active, true, false
					return
				}
			default:
				m.open = false
				return
			}
		}
	}

	if gtx.Focused(m) {
		m.focused = true
	} else if !m.focused && m.focusTries < 4 {
		gtx.Execute(key.FocusCmd{Tag: m})
		m.focusTries++
		gtx.Execute(op.InvalidateCmd{})
	}

	rec := op.Record(gtx.Ops)

	area := clip.Rect{Min: image.Pt(-catcherReach, -catcherReach), Max: image.Pt(catcherReach, catcherReach)}.Push(gtx.Ops)
	event.Op(gtx.Ops, m)
	area.Pop()

	t := m.appear.Animate(gtx, 1, Fast+40*time.Millisecond)
	off := op.Offset(k.clampMenu(gtx, m).Add(image.Pt(0, int(-4*(1-t))))).Push(gtx.Ops)
	sc := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(0.98+0.02*t, 0.98+0.02*t))).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, t)
	m.size = k.menuCard(gtx, m).Size
	opacity.Pop()
	sc.Pop()
	off.Pop()

	op.Defer(gtx.Ops, rec.Stop())
}

func (k *Kit) menuCard(gtx C, m *Menu) D {
	th := k.Th
	width := m.Width

	if width == 0 {
		width = menuWidth
	}

	gtx.Constraints = layout.Constraints{Min: image.Pt(gtx.Dp(width), 0), Max: image.Pt(gtx.Dp(width), 1<<20)}

	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(6).Layout(gtx, func(gtx C) D {
		children := make([]layout.FlexChild, len(m.Items))

		for i := range m.Items {
			children[i] = layout.Rigid(func(gtx C) D {
				return k.menuItem(gtx, &m.clks[i], m.Items[i], m.active == i)
			})
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := rec.Stop()

	radius := gtx.Dp(th.RadiusLarge)
	r := image.Rectangle{Max: dims.Size}
	k.SurfaceRect(gtx, r, radius, th.Popup, true)

	area := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, &m.card)
	area.Pop()

	call.Add(gtx.Ops)
	return dims
}

func (k *Kit) menuItem(gtx C, clk *widget.Clickable, it MenuItem, active bool) D {
	th := k.Th

	if it.Label == "" {
		return layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx C) D {
			return k.Divider(gtx, false)
		})
	}

	return clk.Layout(gtx, func(gtx C) D {
		h := gtx.Dp(menuItemH)
		gtx.Constraints.Min.Y = h
		fg := th.Text

		if it.Danger {
			fg = th.Danger
		}

		if it.Disabled {
			fg = th.TextFaint
		}

		hv := k.Hover(gtx, Key(clk, keyHover), (clk.Hovered() || active) && !it.Disabled)
		bg := Fade(th.Selection, hv)

		if hv > 0 && !it.Disabled && !it.Danger {
			fg = Lerp(fg, th.Text, hv)
		}

		dims := Background(gtx, bg, th.RadiusSmall, func(gtx C) D {
			gtx.Constraints.Min.Y = h
			return layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						col := image.Pt(gtx.Dp(24), 0)

						if it.Icon != nil {
							col.Y = it.Icon.Layout(gtx, 16, fg).Size.Y
						}

						return D{Size: col}
					}),
					layout.Flexed(1, func(gtx C) D {
						gtx.Constraints.Min.Y = h
						return layout.W.Layout(gtx, func(gtx C) D {
							return k.Label(gtx, TextStyle{Color: fg}, it.Label)
						})
					}),
					layout.Rigid(func(gtx C) D {
						if it.Shortcut == "" {
							return D{}
						}

						return k.Kbd(gtx, it.Shortcut)
					}),
				)
			})
		})

		if !it.Disabled {
			Cursor(gtx, dims.Size, pointer.CursorPointer)
		}

		return dims
	})
}

func (k *Kit) shadow(gtx C, size image.Point, radius int) {
	base := color.NRGBA{A: 0x1c}

	if !k.Th.Dark {
		base = color.NRGBA{A: 0x0a}
	}

	for i := 1; i <= 3; i++ {
		d := gtx.Dp(unit.Dp(i * 3))
		r := image.Rect(d/3, d/2, size.X-d/3, size.Y+d)
		RRect(gtx, r, radius+d/2, base)
	}
}

type Select struct {
	Value int
	btn   widget.Clickable
	menu  Menu
}

func (s *Select) Update(gtx C) bool {
	if i, ok := s.menu.Chosen(); ok && i != s.Value {
		s.Value = i
		return true
	}

	return false
}

func (k *Kit) Select(gtx C, s *Select, options []string) D {
	th := k.Th
	s.Update(gtx)

	if s.btn.Clicked(gtx) {
		items := make([]MenuItem, len(options))

		for i, o := range options {
			items[i] = MenuItem{Label: o}

			if i == s.Value {
				items[i].Icon = icons.Check
			}
		}

		s.menu.Open(items, image.Pt(0, gtx.Dp(th.FieldHeight)+gtx.Dp(2)))
	}

	label := ""

	if s.Value >= 0 && s.Value < len(options) {
		label = options[s.Value]
	}

	dims := s.btn.Layout(gtx, func(gtx C) D {
		h := gtx.Dp(th.FieldHeight)
		w := gtx.Constraints.Max.X

		if gtx.Constraints.Min.X > 0 {
			w = gtx.Constraints.Min.X
		}

		size := image.Pt(w, h)
		r := image.Rectangle{Max: size}
		RRect(gtx, r, gtx.Dp(th.RadiusSmall), th.Field)

		line := th.FieldLine

		if s.menu.IsOpen() {
			line = th.TextFaint
		}

		Border(gtx, r, gtx.Dp(th.RadiusSmall), 1, line)

		gtx.Constraints = layout.Exact(size)
		layout.Inset{Left: 8, Right: 4}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					gtx.Constraints.Min.Y = h
					return layout.W.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, TextStyle{}, label)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return icons.ExpandMore.Layout(gtx, 18, th.TextMuted)
				}),
			)
		})
		Cursor(gtx, size, pointer.CursorPointer)
		return D{Size: size}
	})

	s.menu.Width = unit.Dp(float32(dims.Size.X) / gtx.Metric.PxPerDp)
	k.Menu(gtx, &s.menu)
	return dims
}

type Dialog struct {
	Docked bool

	close  widget.Clickable
	list   layout.List
	card   bool
	appear Anim
}

type DialogOpts struct {
	Title    string
	Subtitle string
	Width    unit.Dp
	Body     W
	Footer   W
}

func (k *Kit) dialogTitle(gtx C, o DialogOpts, size unit.Sp) D {
	th := k.Th

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, TextStyle{Size: size, Weight: font.SemiBold}, o.Title)
		}),
		layout.Rigid(func(gtx C) D {
			if o.Subtitle == "" {
				return D{}
			}

			return layout.Inset{Top: 4}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, o.Subtitle)
			})
		}),
	)
}

func (k *Kit) Dialog(gtx C, d *Dialog, o DialogOpts) bool {
	th := k.Th
	closed := d.close.Clicked(gtx)

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})

		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			closed = true
		}
	}

	if d.Docked {
		k.dockedDialog(gtx, d, o)
		return closed
	}

	size := gtx.Constraints.Max

	if !d.appear.init {
		d.appear.Set(0)
	}

	t := d.appear.Animate(gtx, 1, Normal)

	if k.Exit > 0 {
		t = k.Exit * k.Exit
	}
	Rect(gtx, image.Rectangle{Max: size}, Fade(th.Scrim, t))

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, d)
	area.Pop()

	width := o.Width

	if width == 0 {
		width = 480
	}

	gtx.Constraints = layout.Exact(size)
	layout.Center.Layout(gtx, func(gtx C) D {
		w := min(gtx.Dp(width), size.X-gtx.Dp(32))
		gtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, size.Y-gtx.Dp(48))}

		rec := op.Record(gtx.Ops)
		dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: 24, Right: 14, Top: 18, Bottom: 4}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return k.dialogTitle(gtx, o, th.TextSizeLarge+1)
						}),
						layout.Rigid(func(gtx C) D {
							return k.IconButton(gtx, &d.close, icons.Close, false)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: 24, Right: 24, Top: 10, Bottom: 18}.Layout(gtx, o.Body)
			}),
			layout.Rigid(func(gtx C) D {
				if o.Footer == nil {
					return D{}
				}

				return layout.Inset{Left: 24, Right: 24, Top: 4, Bottom: 20}.Layout(gtx, func(gtx C) D {
					return layout.E.Layout(gtx, o.Footer)
				})
			}),
		)
		call := rec.Stop()

		center := f32.Pt(float32(dims.Size.X)/2, float32(dims.Size.Y)/2)
		s := 0.95 + 0.05*t
		tr := f32.Affine2D{}.Scale(center, f32.Pt(s, s)).Offset(f32.Pt(0, float32(gtx.Dp(10))*(1-t)))
		aff := op.Affine(tr).Push(gtx.Ops)
		opacity := paint.PushOpacity(gtx.Ops, t)

		radius := gtx.Dp(th.RadiusLarge + 4)
		r := image.Rectangle{Max: dims.Size}
		k.SurfaceRect(gtx, r, radius, th.Popup, true)
		area := clip.Rect(r).Push(gtx.Ops)
		event.Op(gtx.Ops, &d.card)
		area.Pop()
		call.Add(gtx.Ops)

		opacity.Pop()
		aff.Pop()
		return dims
	})

	return closed
}

func Buttons(ws ...W) W {
	return func(gtx C) D {
		gap := gtx.Dp(8)
		total := gap * (len(ws) - 1)
		measure := gtx
		measure.Constraints.Min = image.Point{}

		for _, w := range ws {
			m := op.Record(gtx.Ops)
			total += w(measure).Size.X
			m.Stop()
		}

		if total <= gtx.Constraints.Max.X {
			children := make([]layout.FlexChild, 0, len(ws)*2)

			for i, w := range ws {
				if i > 0 {
					children = append(children, layout.Rigid(Gap(8)))
				}

				children = append(children, layout.Rigid(w))
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		}

		children := make([]layout.FlexChild, 0, len(ws)*2)

		for i := len(ws) - 1; i >= 0; i-- {
			w := ws[i]

			if i < len(ws)-1 {
				children = append(children, layout.Rigid(Gap(8)))
			}

			children = append(children, layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return w(gtx)
			}))
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

func (m *Menu) step(dir int) {
	n := len(m.Items)

	if n == 0 {
		return
	}

	i := m.active

	if i < 0 && dir < 0 {
		i = n
	}

	for range n {
		i = (i + dir + n) % n

		if it := m.Items[i]; it.Label != "" && !it.Disabled {
			m.active = i
			return
		}
	}
}

func (k *Kit) clampMenu(gtx C, m *Menu) image.Point {
	pos := m.pos

	if !m.known || m.size == (image.Point{}) {
		return pos
	}

	margin := gtx.Dp(8)
	g := m.origin.Add(pos)

	if over := g.X + m.size.X - (k.window.X - margin); over > 0 {
		pos.X -= over
	}

	if g := m.origin.X + pos.X; g < margin {
		pos.X += margin - g
	}

	if g.Y+m.size.Y > k.window.Y-margin {
		pos.Y = max(pos.Y-m.size.Y-gtx.Dp(36), margin-m.origin.Y)
	}

	return pos
}

func (k *Kit) dockedDialog(gtx C, d *Dialog, o DialogOpts) D {
	th := k.Th

	if !d.appear.init {
		d.appear.Set(0)
	}

	t := d.appear.Animate(gtx, 1, Normal)
	size := gtx.Constraints.Max
	defer paint.PushOpacity(gtx.Ops, t).Pop()
	defer op.Offset(image.Pt(0, int(8*(1-t)))).Push(gtx.Ops).Pop()

	k.narrow = true
	defer func() { k.narrow = false }()

	d.list.Axis = layout.Vertical
	gtx.Constraints = layout.Exact(size)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 12, Right: 4, Top: 4, Bottom: 16}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return k.dialogTitle(gtx, o, th.TextSizeLarge)
					}),
					layout.Rigid(func(gtx C) D {
						return k.IconButtonSized(gtx, &d.close, icons.Close, false, 28, 16, th.TextMuted)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			return d.list.Layout(gtx, 1, func(gtx C, _ int) D {
				return layout.Inset{Left: 12, Right: 12, Bottom: 12}.Layout(gtx, o.Body)
			})
		}),
		layout.Rigid(func(gtx C) D {
			if o.Footer == nil {
				return D{}
			}

			return layout.Inset{Left: 12, Right: 12, Top: 8, Bottom: 12}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, o.Footer)
			})
		}),
	)
}

func (k *Kit) Narrow() bool { return k.narrow }
