package palette

import (
	"image"
	"sort"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/plattyssh/platty/internal/ui/icons"
	"github.com/plattyssh/platty/internal/ui/kit"
	"github.com/plattyssh/platty/internal/ui/theme"
)

const (
	paletteWidth   = unit.Dp(640)
	paletteMaxRows = 12
)

type Entry struct {
	Section string
	Title   string
	Detail  string
	Icon    *icons.Icon
	Run     func()
	Score   int
}

type Palette struct {
	Area    image.Rectangle
	k       *kit.Kit
	entries func() []Entry
	quick   func(q string) (Entry, bool)
	onClose func()
	open    bool
	focus   bool
	editor  widget.Editor
	sel     int
	results []Entry
	query   string
	rows    []widget.Clickable
	list    layout.List
	scrim   bool
	hover   int
	appear  kit.Anim
}

func (p *Palette) Show() {
	p.open = true
	p.focus = true
	p.editor.SingleLine = true
	p.editor.Submit = true
	p.editor.SetText("")
	p.query = "\x00"
	p.sel = 0
	p.hover = -1
}

func (p *Palette) Close() {
	p.open = false
}

func (p *Palette) search(q string) {
	p.list.Position = layout.Position{}
	all := p.entries()
	p.results = p.results[:0]

	if e, ok := p.quick(q); ok {
		p.results = append(p.results, e)
	}

	for _, e := range all {
		s := max(fuzzy(e.Title, q), fuzzy(e.Detail, q)/2)

		if s <= 0 {
			continue
		}

		e.Score = s
		p.results = append(p.results, e)
	}

	order := map[string]int{"Quick connect": 0, "Hosts": 1, "Snippets": 2, "Actions": 3}
	sort.SliceStable(p.results, func(i, j int) bool {
		a, b := p.results[i], p.results[j]

		if q == "" || order[a.Section] != order[b.Section] {
			return order[a.Section] < order[b.Section]
		}

		return a.Score > b.Score
	})

	if len(p.results) > 60 {
		p.results = p.results[:60]
	}

	p.sel = 0
}

func (p *Palette) runSelected() {
	if p.sel < 0 || p.sel >= len(p.results) {
		return
	}

	run := p.results[p.sel].Run
	p.open = false
	run()
}

func (p *Palette) Layout(gtx C) {
	k := p.k
	th := k.Th

	if q := p.editor.Text(); q != p.query {
		p.query = q
		p.search(q)
	}

	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &p.editor, Name: key.NameEscape},
			key.Filter{Focus: &p.editor, Name: key.NameUpArrow},
			key.Filter{Focus: &p.editor, Name: key.NameDownArrow},
			pointer.Filter{Target: &p.scrim, Kinds: pointer.Press},
		)

		if !ok {
			break
		}

		switch e := ev.(type) {
		case pointer.Event:
			p.open = false
			p.onClose()
			return
		case key.Event:
			if e.State != key.Press {
				continue
			}

			switch e.Name {
			case key.NameEscape:
				p.open = false
				p.onClose()
				return
			case key.NameUpArrow:
				p.sel = max(p.sel-1, 0)
				p.reveal()
			case key.NameDownArrow:
				p.sel = min(p.sel+1, len(p.results)-1)
				p.reveal()
			}
		}
	}

	for {
		ev, ok := p.editor.Update(gtx)

		if !ok {
			break
		}

		if _, ok := ev.(widget.SubmitEvent); ok {
			p.runSelected()
			p.onClose()
			return
		}
	}

	if len(p.rows) < len(p.results) {
		p.rows = make([]widget.Clickable, len(p.results)+16)
	}

	for i := range p.results {
		if p.rows[i].Clicked(gtx) {
			p.sel = i
			p.runSelected()
			return
		}
	}

	if p.focus {
		gtx.Execute(key.FocusCmd{Tag: &p.editor})
		p.focus = false
	}

	size := gtx.Constraints.Max
	ap := p.appear.Animate(gtx, 1, kit.Normal)
	kit.Rect(gtx, image.Rectangle{Max: size}, kit.Fade(th.Scrim, ap*0.8))
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &p.scrim)
	area.Pop()

	box := p.Area

	if box.Empty() {
		box = image.Rectangle{Max: size}
	}

	w := min(gtx.Dp(paletteWidth), box.Dx()-gtx.Dp(32))
	x := box.Min.X + (box.Dx()-w)/2
	y := box.Min.Y + box.Dy()/6 - int(float32(gtx.Dp(8))*(1-ap))
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	defer paint.PushOpacity(gtx.Ops, ap).Pop()

	gtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, size.Y-y-gtx.Dp(40))}

	rec := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
				return k.Field(gtx, &p.editor, kit.FieldOpts{Hint: "Search hosts, snippets or actions · user@host to connect", Icon: icons.Search})
			})
		}),
		layout.Rigid(func(gtx C) D {
			if len(p.results) == 0 {
				return layout.UniformInset(16).Layout(gtx, func(gtx C) D { return k.Muted(gtx, "No results") })
			}

			rowH := gtx.Dp(30)
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, rowH*paletteMaxRows+gtx.Dp(40))
			p.list.Axis = layout.Vertical
			return layout.Inset{Top: 4, Bottom: 6}.Layout(gtx, func(gtx C) D {
				return p.list.Layout(gtx, len(p.results), func(gtx C, i int) D {
					return p.row(gtx, i)
				})
			})
		}),
	)
	call := rec.Stop()

	r := image.Rectangle{Max: dims.Size}
	radius := gtx.Dp(th.RadiusLarge)
	kit.RRect(gtx, r.Add(image.Pt(0, gtx.Dp(6))), radius, theme.WithAlpha(th.Scrim, 0x50))
	kit.RRect(gtx, r, radius, th.Popup)
	kit.Border(gtx, r, radius, 1, th.Divider)
	cardArea := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, &p.list)
	cardArea.Pop()
	call.Add(gtx.Ops)
}

func (p *Palette) row(gtx C, i int) D {
	k := p.k
	th := k.Th
	e := p.results[i]

	header := i == 0 || p.results[i-1].Section != e.Section

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if !header {
				return D{}
			}

			return layout.Inset{Left: 16, Top: 8, Bottom: 4}.Layout(gtx, func(gtx C) D {
				return k.SectionTitle(gtx, e.Section)
			})
		}),
		layout.Rigid(func(gtx C) D {
			if hv := p.rows[i].Hovered(); hv && p.hover != i {
				p.hover, p.sel = i, i
			} else if !hv && p.hover == i {
				p.hover = -1
			}

			return p.rows[i].Layout(gtx, func(gtx C) D {
				h := gtx.Dp(30)
				size := image.Pt(gtx.Constraints.Max.X, h)

				if i == p.sel {
					kit.RRect(gtx, image.Rect(gtx.Dp(6), 0, size.X-gtx.Dp(6), h), gtx.Dp(th.RadiusSmall), th.Selection)
				}

				gtx.Constraints = layout.Exact(size)
				layout.Inset{Left: 16, Right: 16}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return e.Icon.Layout(gtx, 16, th.TextMuted) }),
						layout.Rigid(kit.Gap(10)),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Max.X = gtx.Constraints.Max.X * 6 / 10
							return k.Text(gtx, e.Title)
						}),
						layout.Rigid(kit.Gap(10)),
						layout.Flexed(1, func(gtx C) D {
							if e.Section == "Actions" {
								return D{Size: gtx.Constraints.Min}
							}

							return k.Label(gtx, kit.TextStyle{Color: th.TextFaint, Size: th.TextSizeSmall, Mono: e.Section == "Snippets"}, e.Detail)
						}),
						layout.Rigid(func(gtx C) D {
							if e.Section != "Actions" || e.Detail == "" {
								return D{}
							}

							return k.Label(gtx, kit.TextStyle{Color: th.TextFaint, Size: th.TextSizeSmall}, e.Detail)
						}),
					)
				})
				kit.Cursor(gtx, size, pointer.CursorPointer)
				return D{Size: size}
			})
		}),
	)
}

func (p *Palette) reveal() {
	pos := p.list.Position

	if p.sel < pos.First || p.sel == pos.First && pos.Offset > 0 {
		p.list.ScrollTo(p.sel)
		return
	}

	last := pos.First + pos.Count - 1

	if pos.OffsetLast < 0 {
		last--
	}

	if p.sel > last {
		p.list.ScrollBy(float32(p.sel - last))
	}
}

func fuzzy(s, q string) int {
	s, q = strings.ToLower(s), strings.ToLower(q)

	if q == "" {
		return 1
	}

	if strings.HasPrefix(s, q) {
		return 1000 - len(s)
	}

	if i := strings.Index(s, q); i >= 0 {
		return 500 - i
	}

	score, si := 100, 0

	for _, r := range q {
		j := strings.IndexRune(s[si:], r)

		if j < 0 {
			return 0
		}

		score -= j
		si += j + len(string(r))
	}

	return max(score, 1)
}

func New(k *kit.Kit, entries func() []Entry, quick func(q string) (Entry, bool), onClose func()) *Palette {
	return &Palette{k: k, entries: entries, quick: quick, onClose: onClose}
}

func (p *Palette) IsOpen() bool {
	return p.open
}

func (p *Palette) Selected() int {
	return p.sel
}

func (p *Palette) Scroll() layout.Position {
	return p.list.Position
}

type (
	C = layout.Context
	D = layout.Dimensions
)

func (p *Palette) SetQuery(q string) {
	p.editor.SetText(q)
}
