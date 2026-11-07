package termview

import (
	"regexp"
	"testing"

	"github.com/northlighthq/vt"
)

func rowCells(text string) []vt.Cell {
	term := vt.New(40, 2)
	term.Write([]byte(text))
	var f vt.Frame
	term.Snapshot(&f, 0)
	return f.Row(0)
}

func TestMarksColorMatches(t *testing.T) {
	red := vt.Indexed(9)
	v := &View{}
	v.marks = []Mark{{Re: regexp.MustCompile("(?i)deploy done"), Color: red}}

	cells := rowCells("ok: \x1b[32mDeploy\x1b[0m done now")
	out := v.highlight(cells)

	for x, c := range out {
		inside := x >= 4 && x < 15

		if inside != (c.Bg == red) {
			t.Fatalf("cell %d (%q) bg = %v, inside = %v", x, c.Rune, c.Bg, inside)
		}
	}

	if cells[4].Bg == red {
		t.Fatal("highlighting must not change the terminal's own cells")
	}
}

func TestMarksOffLeaveRowsAlone(t *testing.T) {
	v := &View{}
	cells := rowCells("deploy done")

	if out := v.highlight(cells); &out[0] != &cells[0] {
		t.Fatal("without marks or highlighting the row should pass through")
	}
}
