package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCdCommand(t *testing.T) {
	if got, want := cdCommand("/srv/it's here"), ` cd -- '/srv/it'"'"'s here'`+"\r"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLocalDir(t *testing.T) {
	dir := t.TempDir()

	if got := localDir(filepath.ToSlash(dir)); got != dir && got != filepath.Clean(dir) {
		t.Fatalf("localDir(%q) = %q", dir, got)
	}

	if vol := filepath.VolumeName(dir); vol != "" {
		if got := localDir("/" + filepath.ToSlash(dir)); got != filepath.Clean(dir) {
			t.Fatalf("OSC 7 drive path: got %q", got)
		}
	}

	if got := localDir(filepath.Join(dir, "missing")); got != "" {
		t.Fatalf("missing folders must be ignored, got %q", got)
	}

	f := filepath.Join(dir, "file")
	os.WriteFile(f, nil, 0o600)

	if got := localDir(f); got != "" {
		t.Fatalf("files must be ignored, got %q", got)
	}
}
