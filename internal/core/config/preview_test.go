package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewUsesItsOwnFolder(t *testing.T) {
	base := t.TempDir()
	t.Setenv("APPDATA", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("HOME", base)

	cfg, err := os.UserConfigDir()

	if err != nil {
		t.Fatal(err)
	}

	real := filepath.Join(cfg, appDir, "vault.dat")
	mustWrite(t, real, "real vault")

	preview = true
	t.Cleanup(func() { preview = false })

	dir, err := Dir()

	if err != nil {
		t.Fatal(err)
	}

	if dir != filepath.Join(cfg, previewDir) {
		t.Fatalf("preview dir = %q, want %q", dir, filepath.Join(cfg, previewDir))
	}

	if b, err := os.ReadFile(real); err != nil || string(b) != "real vault" {
		t.Fatalf("the real vault was touched: %q, %v", b, err)
	}
}
