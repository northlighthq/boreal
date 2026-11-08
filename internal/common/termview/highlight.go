package termview

import (
	"regexp"
	"strings"

	"github.com/northlighthq/vt"
)

var highlightRe = regexp.MustCompile(strings.Join([]string{
	`(?P<url>\bhttps?://[^\s"'<>()\[\]]+)`,
	`(?P<ip>\b(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?(?::\d{1,5})?\b|\b(?:[0-9a-fA-F]{1,4}:){3,7}[0-9a-fA-F]{1,4}\b|::1\b)`,
	`(?P<date>\b\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2})?)?\b|\b\d{2}:\d{2}:\d{2}\b)`,
	`(?P<bad>(?i)\b(?:errors?|err|failed|failure|fatal|denied|refused|panic|critical|crit|timeout|timed out|unreachable|killed|not found)\b)`,
	`(?P<warn>(?i)\b(?:warn|warning|warnings|deprecated|retrying)\b)`,
	`(?P<good>(?i)\b(?:ok|success|successful|succeeded|done|passed|active|running|enabled|connected|listening|up to date)\b)`,
	`(?P<path>(?:^|[\s=:])(?:~|\.{1,2})?/[\w.@+-]+(?:/[\w.@+-]*)+)`,
}, "|"))

var highlightColors = func() []vt.Color {
	byName := map[string]vt.Color{
		"url": vt.Indexed(12), "ip": vt.Indexed(14), "date": vt.Indexed(13),
		"bad": vt.Indexed(9), "warn": vt.Indexed(11), "good": vt.Indexed(10), "path": vt.Indexed(4),
	}
	out := make([]vt.Color, len(highlightRe.SubexpNames()))

	for i, n := range highlightRe.SubexpNames() {
		out[i] = byName[n]
	}

	return out
}()

var markInk = vt.RGB(16, 16, 16)

type Mark struct {
	Re    *regexp.Regexp
	Color vt.Color
}

func (v *View) SetMarks(marks []Mark) {
	v.marks = marks
	v.cache.invalidate()
}

func plain(c vt.Cell) bool {
	return c.Fg.Kind() == vt.ColorDefault && c.Bg.Kind() == vt.ColorDefault && c.Attr&(vt.AttrInverse|vt.AttrInvisible) == 0 && c.Link == 0
}

func (v *View) highlight(cells []vt.Cell) []vt.Cell {
	if (!v.set.Highlight && len(v.marks) == 0) || v.frame.AltScreen {
		return cells
	}

	end := len(cells)

	for end > 0 && (cells[end-1].Rune == 0 || cells[end-1].Rune == ' ') {
		end--
	}

	var sb strings.Builder
	sb.Grow(end)
	offs := v.hlOffs[:0]
	any := false

	for i, c := range cells[:end] {
		r := c.Rune

		if r == 0 {
			r = ' '
		}

		if r > ' ' && plain(c) {
			any = true
		}

		for range len(string(r)) {
			offs = append(offs, i)
		}

		sb.WriteRune(r)
	}

	offs = append(offs, end)
	v.hlOffs = offs
	s := sb.String()
	out := cells
	copied := false
	own := func() {
		if !copied {
			out = append(v.hlCells[:0], cells...)
			copied = true
		}
	}

	if v.set.Highlight && any {
		for _, m := range highlightRe.FindAllStringSubmatchIndex(s, -1) {
			for g := 2; g+1 < len(m); g += 2 {
				if m[g] < 0 {
					continue
				}

				start, end := m[g], m[g+1]

				if g/2 == highlightRe.SubexpIndex("path") {
					start += len(s[start:end]) - len(strings.TrimLeft(s[start:end], " 	=:"))
				}

				own()
				col := highlightColors[g/2]

				for x := offs[start]; x < offs[end]; x++ {
					if plain(out[x]) {
						out[x].Fg = col
					}
				}

				break
			}
		}
	}

	for _, mk := range v.marks {
		for _, m := range mk.Re.FindAllStringIndex(s, -1) {
			if m[0] == m[1] {
				continue
			}

			own()

			for x := offs[m[0]]; x < offs[m[1]]; x++ {
				out[x].Fg, out[x].Bg = markInk, mk.Color
				out[x].Attr &^= vt.AttrInverse | vt.AttrInvisible
			}
		}
	}

	if copied {
		v.hlCells = out
	}

	return out
}
