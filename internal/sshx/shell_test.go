package sshx

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Fatalf("got %s", got)
	}
}

func TestTmuxCommand(t *testing.T) {
	got := TmuxCommand("a.b:c", "")

	if !strings.HasPrefix(got, "exec sh -c '") || !strings.Contains(got, "a_b_c") || strings.Contains(got, "send-keys") {
		t.Fatalf("unexpected command: %s", got)
	}

	if !strings.Contains(TmuxCommand("main", "cd /srv"), "send-keys") {
		t.Fatal("the startup command must be sent to a new session")
	}
}

func TestTmuxCommandParses(t *testing.T) {
	sh, err := exec.LookPath("sh")

	if err != nil {
		t.Skip("sh not available")
	}

	check := strings.Replace(TmuxCommand("it's", "echo 'hi'"), "exec sh -c ", "exec sh -n -c ", 1)
	out, err := exec.Command(sh, "-c", check).CombinedOutput()

	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
