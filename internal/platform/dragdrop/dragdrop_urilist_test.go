//go:build linux && !android && cgo

package dragdrop

import (
	"slices"
	"strings"
	"testing"
)

func TestParseURIList(t *testing.T) {
	in := "# comment\r\nfile:///home/a/b%20c.txt\r\nfile://localhost/tmp/x\r\nhttps://example.com/y\r\nfile://otherhost/z\r\n\r\n"
	want := []string{"/home/a/b c.txt", "/tmp/x"}

	if got := parseURIList(strings.NewReader(in)); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func FuzzParseURIList(f *testing.F) {
	f.Add("file:///home/a/b%20c.txt\r\nfile://localhost/tmp/x\r\n")
	f.Add("# comment\nhttps://example.com\n")

	f.Fuzz(func(t *testing.T, in string) {
		for _, p := range parseURIList(strings.NewReader(in)) {
			if p == "" {
				t.Fatalf("got an empty path from %q, want none", in)
			}
		}
	})
}
