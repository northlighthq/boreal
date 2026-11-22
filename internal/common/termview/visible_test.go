package termview

import (
	"testing"

	"github.com/northlighthq/vt"
)

func TestVisibleTextDropsBlankRows(t *testing.T) {
	term := vt.New(20, 6)
	term.Write([]byte("one   \r\ntwo\r\n"))
	v := &View{backend: benchBackend{term}}
	term.Snapshot(&v.frame, 0)

	if got := v.VisibleText(); got != "one\ntwo" {
		t.Fatalf("got %q, want the two lines without padding or blank rows", got)
	}
}
