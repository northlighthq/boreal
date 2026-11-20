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

	real := filepath.Join(base, appDir)
	os.MkdirAll(real, 0o700)
	os.WriteFile(filepath.Join(real, "vault.dat"), []byte("real vault"), 0o600)

	preview = true
	t.Cleanup(func() { preview = false })

	dir, err := Dir()

	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(dir) != previewDir {
		t.Fatalf("preview dir = %q, want the %q folder", dir, previewDir)
	}

	if b, _ := os.ReadFile(filepath.Join(real, "vault.dat")); string(b) != "real vault" {
		t.Fatal("the real vault was touched")
	}
}
