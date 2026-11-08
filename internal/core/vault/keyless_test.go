package vault

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func keyless(t *testing.T) (*Vault, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "vault.dat")
	v, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	forget(v.env.OS)
	v.env.OS = nil
	return v, path
}

func TestKeylessVaultNeedsPassword(t *testing.T) {
	v, path := keyless(t)

	if !v.NeedsPassword() {
		t.Fatal("a vault without a keyring or password must ask for a password")
	}

	if _, err := v.SaveHost(Host{Address: "example.com"}); !errors.Is(err, ErrNeedPassword) {
		t.Fatalf("saving must refuse to write an unprotected vault, got %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("nothing should reach the disk before a password exists")
	}

	if err := v.SetPassword("", testPassword); err != nil {
		t.Fatal(err)
	}

	if v.NeedsPassword() {
		t.Fatal("the password protects the vault now")
	}

	if _, err := v.SaveHost(Host{Address: "example.com"}); err != nil {
		t.Fatal(err)
	}

	again, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if !again.Locked() {
		t.Fatal("a password-only vault opens locked")
	}

	if err := again.Unlock(testPassword); err != nil {
		t.Fatal(err)
	}

	if len(again.Snapshot().Hosts) != 1 {
		t.Fatal("the host should survive the round trip")
	}
}

func TestPlainOSKeyIsExposed(t *testing.T) {
	v, _ := keyless(t)
	v.env.OS = []byte("0123456789abcdef0123456789abcdef")

	if plainOS(v.env.OS) && !v.NeedsPassword() {
		t.Fatal("a key stored in the clear must count as unprotected")
	}
}

func TestPasswordChangeRotatesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.dat")
	v, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if err := v.SetPassword("", testPassword); err != nil {
		t.Fatal(err)
	}

	old := append([]byte(nil), v.key...)
	oldFile, err := os.ReadFile(path)

	if err != nil {
		t.Fatal(err)
	}

	if err := v.SetPassword(testPassword, "a brand new password"); err != nil {
		t.Fatal(err)
	}

	if string(old) == string(v.key) {
		t.Fatal("changing the password must change the data key")
	}

	stale := filepath.Join(t.TempDir(), "old.dat")

	if err := os.WriteFile(stale, oldFile, 0o600); err != nil {
		t.Fatal(err)
	}

	current, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	current.Lock()

	if err := current.Unlock(testPassword); err == nil {
		t.Fatal("the old password must not open the new vault")
	}

	if err := current.Unlock("a brand new password"); err != nil {
		t.Fatal(err)
	}
}
