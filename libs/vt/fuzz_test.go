package vt

import (
	"testing"
)

var fuzzSeeds = []string{
	"hello, world\r\n",
	"\x1b[31;1mred\x1b[0m \x1b[38;2;10;20;30mtrue\x1b[38:5:200mcolor",
	"\x1b[?1049h\x1b[2J\x1b[H alt \x1b[?1049l",
	"\x1b[10;20H\x1b[5X\x1b[3@\x1b[2P\x1b[4L\x1b[2M\x1b[S\x1b[T",
	"\x1b]0;title\x07\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\",
	"\x1b]52;c;aGVsbG8=\x07\x1b]7;file://host/tmp\x07",
	"漢字かな😀👍🏽́é\r\n",
	"\x1b[?25l\x1b[?1000h\x1b[?2004h\x1b[6n\x1b[c",
	"\x1b[3;5r\x1bD\x1bM\x1b7\x1b8\x1b#8",
	"\t\tTAB\x1bH\x1b[3g\x1b[0g\b\b\x7f",
}

func FuzzTerminal(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add([]byte(s), uint8(80), uint8(24), uint8(40), uint8(10))
	}

	f.Fuzz(func(t *testing.T, data []byte, cols, rows, cols2, rows2 uint8) {
		term := New(int(cols), int(rows), WithScrollback(64))
		var frame Frame

		half := len(data) / 2

		if _, err := term.Write(data[:half]); err != nil {
			t.Fatal(err)
		}

		term.Snapshot(&frame, 0)
		term.Resize(int(cols2), int(rows2))
		term.Snapshot(&frame, 3)

		if _, err := term.Write(data[half:]); err != nil {
			t.Fatal(err)
		}

		term.Snapshot(&frame, 1<<20)
		term.Title()
		term.Modes()

		if len(frame.Cells) != frame.Cols*frame.Rows {
			t.Fatalf("frame has %d cells for %dx%d", len(frame.Cells), frame.Cols, frame.Rows)
		}
	})
}
