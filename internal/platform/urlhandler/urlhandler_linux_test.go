//go:build linux && !android

package urlhandler

import "testing"

func TestQuoteExec(t *testing.T) {
	cases := map[string]string{
		"/opt/Boreal/boreal":        `"/opt/Boreal/boreal"`,
		"/home/a b/Boreal.AppImage": `"/home/a b/Boreal.AppImage"`,
		`/tmp/$x"y`:                 `"/tmp/\\$x\\"y"`,
		`/tmp/back\slash`:           `"/tmp/back\\\\slash"`,
	}

	for in, want := range cases {
		if got := quoteExec(in); got != want {
			t.Errorf("quoteExec(%q) = %s, want %s", in, got, want)
		}
	}
}
