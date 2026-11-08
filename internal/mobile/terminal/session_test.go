package terminal

import "testing"

func TestTransform(t *testing.T) {
	cases := []struct {
		ctrl, alt bool
		in, want  string
	}{
		{false, false, "ls", "ls"},
		{true, false, "c", "\x03"},
		{true, false, "L", "\x0c"},
		{true, false, "[x", "\x1bx"},
		{false, true, "b", "\x1bb"},
		{true, true, "a", "\x1b\x01"},
		{true, false, "é", "é"},
	}

	for _, c := range cases {
		s := &Session{ctrl: c.ctrl, alt: c.alt}

		if got := s.transform(c.in); got != c.want {
			t.Fatalf("ctrl=%v alt=%v %q: got %q, want %q", c.ctrl, c.alt, c.in, got, c.want)
		}

		if s.ctrl || s.alt {
			t.Fatalf("modifiers must release after one key")
		}
	}
}
