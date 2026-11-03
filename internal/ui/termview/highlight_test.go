package termview

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/northlighthq/vt"
)

func (v *View) highlightOld(cells []vt.Cell) []vt.Cell {
	if !v.set.Highlight || v.frame.AltScreen {
		return cells
	}

	var sb strings.Builder
	offs := v.hlOffs[:0]
	any := false

	for i, c := range cells {
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

	offs = append(offs, len(cells))
	v.hlOffs = offs

	if !any {
		return cells
	}

	s := sb.String()
	matches := highlightRe.FindAllStringSubmatchIndex(s, -1)

	if len(matches) == 0 {
		return cells
	}

	out := append(v.hlCells[:0], cells...)

	for _, m := range matches {
		for g := 2; g+1 < len(m); g += 2 {
			if m[g] < 0 {
				continue
			}

			start, end := m[g], m[g+1]

			if g/2 == highlightRe.SubexpIndex("path") {
				start += len(s[start:end]) - len(strings.TrimLeft(s[start:end], " \t=:"))
			}

			col := highlightColors[g/2]

			for x := offs[start]; x < offs[end]; x++ {
				if plain(out[x]) {
					out[x].Fg = col
				}
			}

			break
		}
	}

	v.hlCells = out
	return out
}

func TestHighlightMatchesReference(t *testing.T) {
	words := []string{"error", "ok", "https://example.com/a", "10.0.0.1:22", "2026-11-03", "12:30:00", "/var/log/syslog", "~/src/app", "warning", "timed out", "x", "=", ":", "漢字", "failed", "running"}
	r := rand.New(rand.NewPCG(5, 6))
	v := &View{set: Settings{Highlight: true}}
	ref := &View{set: Settings{Highlight: true}}

	for range 3000 {
		var parts []string

		for range r.IntN(8) {
			parts = append(parts, words[r.IntN(len(words))])
		}

		text := strings.Join(parts, strings.Repeat(" ", 1+r.IntN(2)))
		term := vt.New(80, 1)
		term.Write([]byte(text))
		var f vt.Frame
		term.Snapshot(&f, 0)
		row := f.Row(0)

		got := slices.Clone(v.highlight(row))
		want := ref.highlightOld(row)

		if !slices.Equal(got, want) {
			t.Fatalf("%q highlighted differently", text)
		}
	}
}
