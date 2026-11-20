package config

import (
	"os"
	"path/filepath"
	"testing"
)

func mustWrite(t *testing.T, path, text string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMovePrivateMovesTheData(t *testing.T) {
	base := t.TempDir()
	dir, private := filepath.Join(base, "files", appDir), filepath.Join(base, "no_backup", appDir)
	mustWrite(t, filepath.Join(dir, "vault.dat"), "mine")

	if got := movePrivate(dir, private); got != private {
		t.Fatalf("dir = %q, want %q", got, private)
	}

	if b, err := os.ReadFile(filepath.Join(private, "vault.dat")); err != nil || string(b) != "mine" {
		t.Fatalf("moved vault = %q, %v", b, err)
	}
}

func TestMovePrivateKeepsTheOldFolderWhenTheMoveFails(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "files", appDir)
	blocker := filepath.Join(base, "no_backup")
	mustWrite(t, filepath.Join(dir, "vault.dat"), "mine")
	mustWrite(t, blocker, "a file where the folder should be")

	if got := movePrivate(dir, filepath.Join(blocker, appDir)); got != dir {
		t.Fatalf("a failed move must keep using %q, got %q", dir, got)
	}

	if b, err := os.ReadFile(filepath.Join(dir, "vault.dat")); err != nil || string(b) != "mine" {
		t.Fatalf("the vault must stay where it was: %q, %v", b, err)
	}
}

func TestMovePrivatePrefersAnExistingPrivateFolder(t *testing.T) {
	base := t.TempDir()
	dir, private := filepath.Join(base, "files", appDir), filepath.Join(base, "no_backup", appDir)
	mustWrite(t, filepath.Join(private, "vault.dat"), "moved before")

	if got := movePrivate(dir, private); got != private {
		t.Fatalf("dir = %q, want %q", got, private)
	}

	if got := movePrivate(filepath.Join(base, "missing"), filepath.Join(base, "new", appDir)); got != filepath.Join(base, "new", appDir) {
		t.Fatalf("with nothing to move, the private folder is used: %q", got)
	}
}
