package termview

import (
	"fmt"
	"image"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	searchWidth   = unit.Dp(380)
	searchRefresh = 250 * time.Millisecond
)

type search struct {
	open      bool
	focus     bool
	editor    widget.Editor
	matchCase widget.Bool
	prev      widget.Clickable
	next      widget.Clickable
	close     widget.Clickable

	query    string
	caseSens bool
	matches  []vt.Match
	current  int
	gen      uint64
	lastScan time.Time
}

func (s *search) show() {
	s.open = true
	s.focus = true
	s.editor.SingleLine = true
	s.editor.Submit = true
	s.editor.SetCaret(s.editor.Len(), 0)
}

func (s *search) refresh(v *View) {
	if !s.open {
		return
	}

	q := s.editor.Text()
	gen := v.backend.Terminal().Generation()
	changed := q != s.query || s.matchCase.Value != s.caseSens

	if !changed && (gen == s.gen || time.Since(s.lastScan) < searchRefresh) {
		return
	}

	s.query, s.caseSens, s.gen, s.lastScan = q, s.matchCase.Value, gen, time.Now()
	s.matches = v.backend.Terminal().Find(q, s.caseSens)

	if changed {
		s.current = len(s.matches) - 1
		v.revealMatch()
	}

	s.current = min(s.current, len(s.matches)-1)
}

func (v *View) stepMatch(dir int) {
	s := &v.search
	n := len(s.matches)

	if n == 0 {
		return
	}

	s.current = ((s.current+dir)%n + n) % n
	v.revealMatch()
}

func (v *View) revealMatch() {
	s := &v.search

	if s.current < 0 || s.current >= len(s.matches) {
		return
	}

	f := &v.frame
	base := f.TopLine + f.Offset
	line := s.matches[s.current].Start.Line

	if line >= f.TopLine && line < f.TopLine+f.Rows {
		return
	}

	v.offset = min(max(base-line+f.Rows/2, 0), f.Scrollback)
}

func (v *View) layoutSearchBar(gtx layout.Context, size image.Point) {
	k := v.kit
	th := k.Th
	s := &v.search

	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &s.editor, Name: key.NameEscape},
			key.Filter{Focus: &s.editor, Name: key.NameReturn, Required: key.ModShift},
			key.Filter{Focus: &s.editor, Name: key.NameUpArrow},
			key.Filter{Focus: &s.editor, Name: key.NameDownArrow},
		)

		if !ok {
			break
		}

		e, ok := ev.(key.Event)

		if !ok || e.State != key.Press {
			continue
		}

		switch e.Name {
		case key.NameEscape:
			s.open = false
			v.wantFocus = true
			return
		case key.NameReturn, key.NameDownArrow:
			v.stepMatch(1)
		case key.NameUpArrow:
			v.stepMatch(-1)
		}
	}

	for {
		ev, ok := s.editor.Update(gtx)

		if !ok {
			break
		}

		if _, ok := ev.(widget.SubmitEvent); ok {
			v.stepMatch(-1)
		}
	}

	if s.prev.Clicked(gtx) {
		v.stepMatch(-1)
	}

	if s.next.Clicked(gtx) {
		v.stepMatch(1)
	}

	if s.close.Clicked(gtx) {
		s.open = false
		v.wantFocus = true
		return
	}

	if s.focus {
		gtx.Execute(key.FocusCmd{Tag: &s.editor})
		s.focus = false
	}

	w := min(gtx.Dp(searchWidth), size.X-gtx.Dp(16))
	x := size.X - w - gtx.Dp(16)
	defer op.Offset(image.Pt(x, gtx.Dp(8))).Push(gtx.Ops).Pop()

	gtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Dp(44))}

	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return k.Field(gtx, &s.editor, kit.FieldOpts{
					Hint:    "Search",
					Icon:    icons.Search,
					Invalid: s.query != "" && len(s.matches) == 0,
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := "0 results"

					if n := len(s.matches); n > 0 {
						label = fmt.Sprintf("%d/%d", s.current+1, n)
					}

					return k.Small(gtx, label)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return k.IconButton(gtx, &s.prev, icons.ExpandLess, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return k.IconButton(gtx, &s.next, icons.ExpandMore, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.matchCase.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					fg := th.TextMuted
					var bg = th.Panel
					bg.A = 0

					if s.matchCase.Value {
						fg, bg = th.Accent, th.AccentSoft
					}

					return kit.Background(gtx, bg, th.RadiusSmall, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: 6, Right: 6, Top: 5, Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return k.Label(gtx, kit.TextStyle{Color: fg, Size: th.TextSizeSmall}, "Aa")
						})
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return k.IconButton(gtx, &s.close, icons.Close, false)
			}),
		)
	})
	call := rec.Stop()

	r := image.Rectangle{Max: dims.Size}
	kit.RRect(gtx, r, gtx.Dp(th.Radius), th.Popup)
	kit.Border(gtx, r, gtx.Dp(th.Radius), 1, th.Divider)
	call.Add(gtx.Ops)
}
