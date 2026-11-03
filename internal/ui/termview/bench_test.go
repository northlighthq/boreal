package termview

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

type benchBackend struct{ t *vt.Terminal }

func (b benchBackend) Terminal() *vt.Terminal { return b.t }
func (b benchBackend) Send([]byte)            {}
func (b benchBackend) Resize(cols, rows int)  { b.t.Resize(cols, rows) }

func benchView(b *testing.B) (*View, *vt.Terminal, func() layout.Context) {
	b.Helper()
	set, err := fonts.Glyphs(fonts.TerminalFonts[0])

	if err != nil {
		b.Skip(err)
	}

	shaper, _ := fonts.Load()
	term := vt.New(200, 50)
	v := New(benchBackend{term}, kit.New(theme.New(""), shaper), set, Settings{TextSize: 13, LineHeight: 1.2, Ligatures: true, Highlight: true})
	ops := new(op.Ops)
	now := time.Now()

	gtx := func() layout.Context {
		ops.Reset()
		now = now.Add(16 * time.Millisecond)
		return layout.Context{
			Ops:         ops,
			Now:         now,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(1700, 1000)),
		}
	}

	v.Layout(gtx())
	return v, term, gtx
}

func BenchmarkLayoutIdle(b *testing.B) {
	v, term, gtx := benchView(b)

	for i := range 60 {
		fmt.Fprintf(term, "\x1b[3%dmline %d\x1b[0m some text that fills part of the row %d\r\n", i%8, i, i*31)
	}

	v.Layout(gtx())
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		v.Layout(gtx())
	}
}

func BenchmarkLayoutScrollingLog(b *testing.B) {
	v, term, gtx := benchView(b)
	i := 0
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for range 5 {
			fmt.Fprintf(term, "2026-11-03 INFO request id=%08x path=/api/v1/items/%d status=200\r\n", i*2654435761, i)
			i++
		}

		v.Layout(gtx())
	}
}

func BenchmarkLayoutFullRedraw(b *testing.B) {
	v, term, gtx := benchView(b)
	frame := 0
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		term.Write([]byte("\x1b[H"))

		for y := 1; y <= 50; y++ {
			fmt.Fprintf(term, "\x1b[%d;1H\x1b[%dm%6d cpu %5.1f%% mem %5.1f%% process-%d\x1b[0m\x1b[K", y, 31+(y+frame)%7, frame*y, float64((y*frame)%1000)/10, float64(y)/3, y)
		}

		frame++
		v.Layout(gtx())
	}
}
