//go:build linux && !android && !nowayland && cgo

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
