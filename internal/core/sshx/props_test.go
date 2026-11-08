package sshx

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestModeRoundTrip(t *testing.T) {
	for _, s := range []string{"755", "644", "000", "2775", "4755", "1777", "7777"} {
		m, ok := ParseMode(s)

		if !ok {
			t.Fatalf("ParseMode(%q) failed", s)
		}

		if got := FormatMode(m); got != s {
			t.Errorf("FormatMode(ParseMode(%q)) = %q", s, got)
		}
	}

	for _, s := range []string{"", "75", "888", "12345", "rwx"} {
		if _, ok := ParseMode(s); ok {
			t.Errorf("ParseMode(%q) should fail", s)
		}
	}
}

func TestTreeMode(t *testing.T) {
	cases := []struct {
		dir     bool
		current os.FileMode
		want    os.FileMode
	}{
		{true, 0o700, 0o755},
		{false, 0o644, 0o644},
		{false, 0o700, 0o755},
	}

	for _, c := range cases {
		if got := TreeMode(0o755, c.dir, c.current); got != c.want {
			t.Errorf("TreeMode(755, %v, %o) = %o, want %o", c.dir, c.current, got, c.want)
		}
	}
}

func TestNames(t *testing.T) {
	passwd := "root:x:0:0:root:/root:/bin/bash\n# comment\nbroken\ndeploy:x:1000:1000::/home/deploy:/bin/sh\ndup:x:1000:1000::/:/bin/sh\n"
	n := Names{Users: parseIDs(bufio.NewScanner(strings.NewReader(passwd)), 2)}

	if n.User(0) != "root" || n.User(1000) != "deploy" || n.User(42) != "42" {
		t.Fatalf("unexpected users %v", n.Users)
	}

	if id, ok := n.UserID("deploy"); !ok || id != 1000 {
		t.Fatalf("UserID(deploy) = %d, %v", id, ok)
	}

	if id, ok := n.UserID("33"); !ok || id != 33 {
		t.Fatalf("UserID(33) = %d, %v", id, ok)
	}

	if _, ok := n.UserID("nobody"); ok {
		t.Fatal("unknown names must fail")
	}
}
