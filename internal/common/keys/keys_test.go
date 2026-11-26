package keys

import (
	"runtime"
	"testing"
)

func TestLabel(t *testing.T) {
	want := map[string]string{
		"Ctrl+Shift+T":              "Ctrl+Shift+T",
		"Ctrl+Tab / Ctrl+Shift+Tab": "Ctrl+Tab / Ctrl+Shift+Tab",
		"Ctrl+Alt+S":                "Ctrl+Alt+S",
		"Alt+1 … Alt+6":             "Alt+1 … Alt+6",
	}

	if runtime.GOOS == "darwin" {
		want = map[string]string{
			"Ctrl+Shift+T":              "⌘T",
			"Ctrl+Tab / Ctrl+Shift+Tab": "⌃Tab / ⌃⇧Tab",
			"Ctrl+Alt+S":                "⌘,",
			"Alt+1 … Alt+6":             "⌥1 … ⌥6",
		}
	}

	for in, out := range want {
		if got := Label(in); got != out {
			t.Errorf("Label(%q) = %q, want %q", in, got, out)
		}
	}
}

func TestIsApp(t *testing.T) {
	for _, m := range App {
		if !IsApp(m) {
			t.Errorf("IsApp(%v) = false", m)
		}
	}
}
