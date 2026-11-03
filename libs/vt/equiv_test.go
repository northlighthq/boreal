package vt

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/northlighthq/vt/parser"
)

type plainHandler struct{ t *Terminal }

func (h plainHandler) Print(r rune)                                   { h.t.Print(r) }
func (h plainHandler) Execute(b byte)                                 { h.t.Execute(b) }
func (h plainHandler) CSI(p *parser.Params, inter []byte, final byte) { h.t.CSI(p, inter, final) }
func (h plainHandler) ESC(inter []byte, final byte)                   { h.t.ESC(inter, final) }
func (h plainHandler) OSC(data []byte)                                { h.t.OSC(data) }

var streamPieces = []string{
	"hello world ", "x", "\r\n", "\n", "\r", "\t", "\b", "漢字", "😀", "é",
	"\x1b[31m", "\x1b[0m", "\x1b[1;44m", "\x1b[K", "\x1b[2J", "\x1b[H", "\x1b[5;10H", "\x1b[3C", "\x1b[2D",
	"\x1b[4@", "\x1b[2P", "\x1b[L", "\x1b[M", "\x1b[4h", "\x1b[4l", "\x1b(0lqk\x1b(B", "\x1b[?7l", "\x1b[?7h",
	"\x1b[2;8r", "\x1b[r", "\x1bM", "\x1b[?1049h", "\x1b[?1049l", "\x1b]8;;https://example.com\x1b\\",
	"\x1b]8;;\x1b\\", "\x1b[3X", "\x1b[2b",
}

func randomStream(r *rand.Rand, n int) []byte {
	var b strings.Builder

	for range n {
		if r.IntN(3) == 0 {
			b.WriteString(strings.Repeat(string(rune('a'+r.IntN(26))), 1+r.IntN(120)))
			continue
		}

		b.WriteString(streamPieces[r.IntN(len(streamPieces))])
	}

	return []byte(b.String())
}

func dumpState(t *Terminal) string {
	var b strings.Builder
	fmt.Fprintf(&b, "size %dx%d cur %+v dropped %d sb %d\n", t.cols, t.rows, t.cur, t.sb.dropped, t.sb.len())

	for i := range t.sb.len() {
		l := t.sb.at(i)
		fmt.Fprintf(&b, "sb %v %v\n", l.wrapped, l.cells)
	}

	for _, buf := range []*buffer{t.primary, t.alt} {
		for _, l := range buf.lines {
			fmt.Fprintf(&b, "ln %v %v\n", l.wrapped, l.cells)
		}
	}

	return b.String()
}

func diffLines(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")

	for i := range min(len(al), len(bl)) {
		if al[i] != bl[i] {
			return fmt.Sprintf("line %d:\n%s\n%s", i, al[i], bl[i])
		}
	}

	return fmt.Sprintf("lengths %d vs %d", len(al), len(bl))
}

func TestPrintASCIIMatchesPrint(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))

	for i := range 500 {
		cols, rows := 1+r.IntN(90), 1+r.IntN(30)
		fast := New(cols, rows, WithScrollback(50))
		slow := New(cols, rows, WithScrollback(50))
		slow.parser = parser.New(plainHandler{slow})
		data := randomStream(r, 60)

		for len(data) > 0 {
			n := min(len(data), 1+r.IntN(64))
			fast.Write(data[:n])
			slow.Write(data[:n])
			data = data[n:]
		}

		if a, b := dumpState(fast), dumpState(slow); a != b {
			t.Fatalf("case %d (%dx%d): %s", i, cols, rows, diffLines(a, b))
		}
	}
}

func TestReflowMatchesReference(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))

	for i := range 500 {
		cols, rows := 1+r.IntN(90), 1+r.IntN(30)
		sb := r.IntN(80)
		data := randomStream(r, 80)
		fresh := New(cols, rows, WithScrollback(sb))
		ref := New(cols, rows, WithScrollback(sb))
		fresh.Write(data)
		ref.Write(data)

		for range 4 {
			c, rw := 1+r.IntN(90), 1+r.IntN(30)
			fresh.Resize(c, rw)
			ref.resizeOld(c, rw)

			if a, b := dumpState(fresh), dumpState(ref); a != b {
				t.Fatalf("case %d resize to %dx%d: %s", i, c, rw, diffLines(a, b))
			}

			more := randomStream(r, 10)
			fresh.Write(more)
			ref.Write(more)
		}
	}
}
