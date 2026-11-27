package keys

import "testing"

func TestLabel(t *testing.T) {
	for in, want := range wantLabels {
		if got := Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsApp(t *testing.T) {
	for _, m := range App {
		if !IsApp(m) {
			t.Errorf("IsApp(%v) = false, want true", m)
		}
	}
}
