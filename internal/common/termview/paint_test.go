package termview

import (
	"image"
	"testing"

	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/theme"
)

func TestRowCanvasKeepsEveryPixel(t *testing.T) {
	set, err := fonts.Glyphs(fonts.TerminalFonts[0])

	if err != nil {
		t.Skip(err)
	}

	shaper, _ := fonts.Load()
	rows := []string{
		"plain text",
		"\x1b[3mitalic wwwwWWW\x1b[0m",
		"\x1b[1;3mbold italic fff\x1b[0m",
		"wide 漢字",
		"emoji 😀👍",
		"box ├──┤ ╔═╗ █▓▒░",
		"\x1b[4munderlined\x1b[0m   ",
		"\x1b[4m   \x1b[0m",
		"\x1b[44mblue background\x1b[0m",
		"\x1b[7minverse\x1b[0m",
		"\x1b[9mstrike\x1b[0m",
		"\x1b[44m\x1b[K",
		"trailing spaces      ",
		"",
	}

	for _, size := range []float32{11, 13, 18} {
		for _, text := range rows {
			term := vt.New(40, 2)
			term.Write([]byte(text))
			v := New(benchBackend{term}, kit.New(theme.New(""), shaper), set, Settings{TextSize: 13, LineHeight: 1.2})
			v.glyphs = sharedCache(set, int(size), 1.2)
			gm := v.glyphs.Metrics()
			v.m = metrics{px: int(size), line: 1.2, w: gm.CellW, h: gm.CellH, baseline: gm.Baseline}

			var f vt.Frame
			term.Snapshot(&f, 0)
			cells := f.Row(0)

			full := image.NewRGBA(image.Rect(0, 0, len(cells)*v.m.w, v.m.h))
			v.paintRow(full, v.highlight(cells))

			var e rowEntry
			v.rasterRow(&e, cells)
			w := 0

			if e.img != nil {
				w = e.img.Rect.Dx()
			}

			for y := range v.m.h {
				for x := range full.Rect.Dx() {
					want := full.RGBAAt(x, y)

					if x >= w {
						if want.A != 0 {
							t.Fatalf("size %v %q: pixel %d,%d painted outside the %dpx canvas", size, text, x, y, w)
						}

						continue
					}

					if got := e.img.RGBAAt(x, y); got != want {
						t.Fatalf("size %v %q: pixel %d,%d = %v, want %v", size, text, x, y, got, want)
					}
				}
			}
		}
	}
}
