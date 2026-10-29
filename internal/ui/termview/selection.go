package termview

import (
	"image"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"github.com/northlighthq/vt"
)

type selectMode uint8

const (
	selectChar selectMode = iota
	selectWord
	selectLine
)

type selection struct {
	active            bool
	dragging          bool
	mode              selectMode
	anchor, anchorEnd vt.Pos
	head              vt.Pos

	lastPress time.Time
	lastPos   image.Point
	clicks    int
}

func (s *selection) clear() {
	s.active = false
	s.dragging = false
}

func (s *selection) bounds() (vt.Pos, vt.Pos) {
	a, b := s.anchor, s.head

	if s.mode != selectChar {
		if s.head.Before(s.anchor) {
			return s.head, s.anchorEnd
		}

		return s.anchor, s.head
	}

	if b.Before(a) {
		a, b = b, a
	}

	return a, b
}

func (v *View) pressSelection(e pointer.Event) {
	now := time.Now()
	p := e.Position.Round()

	s := &v.sel

	if now.Sub(s.lastPress) < multiClick && near(p, s.lastPos) {
		s.clicks = s.clicks%3 + 1
	} else {
		s.clicks = 1
	}

	s.lastPress, s.lastPos = now, p

	pos := v.posAt(e.Position)

	if e.Modifiers.Contain(key.ModShift) && s.active {
		s.head = pos
		s.dragging = true
		return
	}

	switch s.clicks {
	case 2:
		start, end := v.wordAt(pos)
		*s = selection{active: true, dragging: true, mode: selectWord, anchor: start, anchorEnd: end, head: end,
			lastPress: now, lastPos: p, clicks: 2}
	case 3:
		start := vt.Pos{Line: pos.Line, Col: 0}
		end := vt.Pos{Line: pos.Line, Col: v.frame.Cols - 1}
		*s = selection{active: true, dragging: true, mode: selectLine, anchor: start, anchorEnd: end, head: end,
			lastPress: now, lastPos: p, clicks: 3}
	default:
		*s = selection{dragging: true, mode: selectChar, anchor: pos, head: pos, lastPress: now, lastPos: p, clicks: 1}
	}
}

func (v *View) dragSelection(e pointer.Event) {
	s := &v.sel

	if !s.dragging {
		return
	}

	_, y := v.cellAt(e.Position)

	switch {
	case y < 0:
		v.offset++
	case y >= v.frame.Rows:
		v.offset = max(v.offset-1, 0)
	}

	pos := v.posAt(e.Position)

	switch s.mode {
	case selectWord:
		start, end := v.wordAt(pos)

		if pos.Before(s.anchor) {
			s.head = start
		} else {
			s.head = end
		}
	case selectLine:
		if pos.Before(s.anchor) {
			s.head = vt.Pos{Line: pos.Line, Col: 0}
		} else {
			s.head = vt.Pos{Line: pos.Line, Col: v.frame.Cols - 1}
		}
	default:
		s.head = pos
	}

	s.active = s.anchor != s.head || s.mode != selectChar
}

func near(a, b image.Point) bool {
	d := a.Sub(b)
	return d.X*d.X+d.Y*d.Y < 25
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_./~:@%+=#?&", r)
}

func (v *View) wordAt(pos vt.Pos) (vt.Pos, vt.Pos) {
	y := pos.Line - v.frame.TopLine

	if y < 0 || y >= v.frame.Rows {
		return pos, pos
	}

	row := v.frame.Row(y)

	if !isWordRune(row[pos.Col].Rune) {
		return pos, pos
	}

	start, end := pos.Col, pos.Col

	for start > 0 && (isWordRune(row[start-1].Rune) || row[start-1].Width() == 0) {
		start--
	}

	for end < len(row)-1 && (isWordRune(row[end+1].Rune) || row[end+1].Width() == 0) {
		end++
	}

	return vt.Pos{Line: pos.Line, Col: start}, vt.Pos{Line: pos.Line, Col: end}
}

type hoverLink struct {
	active bool
	y      int
	x0, x1 int
	url    string
	pos    f32.Point
}

var urlRe = regexp.MustCompile(`https?://[^\s"'<>\x60]+`)

func (v *View) updateHover(p f32.Point, m key.Modifiers) {
	v.hover.pos = p
	v.updateHoverMods(m)
}

func (v *View) updateHoverMods(m key.Modifiers) {
	if !m.Contain(key.ModCtrl) {
		v.hover.active = false
		return
	}

	x, y := v.cellAt(v.hover.pos)

	if y < 0 || y >= v.frame.Rows {
		v.hover.active = false
		return
	}

	row := v.frame.Row(y)

	if x >= 0 && x < len(row) && row[x].Link != 0 {
		id := row[x].Link
		x0, x1 := x, x

		for x0 > 0 && row[x0-1].Link == id {
			x0--
		}

		for x1 < len(row)-1 && row[x1+1].Link == id {
			x1++
		}

		if url := v.backend.Terminal().Link(id); url != "" {
			v.hover = hoverLink{active: true, y: y, x0: x0, x1: x1 + 1, url: url, pos: v.hover.pos}
			return
		}
	}

	runes := make([]rune, 0, len(row))
	cols := make([]int, 0, len(row))

	for i, c := range row {
		if c.Width() == 0 {
			continue
		}

		r := c.Rune

		if r == 0 {
			r = ' '
		}

		runes = append(runes, r)
		cols = append(cols, i)
	}

	line := string(runes)

	for _, loc := range urlRe.FindAllStringIndex(line, -1) {
		start := len([]rune(line[:loc[0]]))
		end := start + len([]rune(line[loc[0]:loc[1]])) - 1

		if x < cols[start] || x > cols[end] {
			continue
		}

		url := strings.TrimRight(line[loc[0]:loc[1]], ".,;:)]}")
		end = start + len([]rune(url)) - 1
		v.hover = hoverLink{active: true, y: y, x0: cols[start], x1: cols[end] + 1, url: url, pos: v.hover.pos}
		return
	}

	v.hover.active = false
}

func (v *View) paintLink(gtx layout.Context) {
	if !v.hover.active {
		return
	}

	y := (v.hover.y+1)*v.m.h - max(v.m.h/8, 2)
	fillRect(gtx, image.Rect(v.hover.x0*v.m.w, y, v.hover.x1*v.m.w, y+1), v.theme.Accent)
}
