package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/northlighthq/boreal/internal/core/secure"
)

const testPassword = "correct horse battery"

func writeLegacy(t *testing.T, withOS bool) (string, []byte) {
	t.Helper()

	key := secure.NewKey()
	plain, err := json.Marshal(Data{Version: version, Hosts: []Host{{ID: "h1", Address: "example.com", Port: 22}}})

	if err != nil {
		t.Fatal(err)
	}

	sealed, err := secure.Seal(key, plain, legacyDataAAD)

	if err != nil {
		t.Fatal(err)
	}

	kdf := secure.NewKDF(1, 8*1024)
	kek, err := kdf.Key(testPassword)

	if err != nil {
		t.Fatal(err)
	}

	wrappedKey, err := secure.Seal(kek, key, legacyKeyAAD)

	if err != nil {
		t.Fatal(err)
	}

	env := envelope{Format: legacyFormat, Version: 2, Password: &wrapped{KDF: kdf, Key: wrappedKey}, Data: sealed}

	if withOS {
		osKey, err := protect(key)

		if err != nil {
			t.Fatal(err)
		}

		env.OS = osKey
	}

	raw, err := json.Marshal(env)

	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), vaultFile)

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, key
}

func readEnvelope(t *testing.T, path string) envelope {
	t.Helper()

	raw, err := os.ReadFile(path)

	if err != nil {
		t.Fatal(err)
	}

	var env envelope

	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}

	return env
}

func TestLegacyVaultWithPasswordUpgrades(t *testing.T) {
	path, _ := writeLegacy(t, false)
	v, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if !v.Locked() {
		t.Fatal("expected a locked vault")
	}

	if err := v.Unlock(testPassword); err != nil {
		t.Fatal(err)
	}

	if _, ok := v.Host("h1"); !ok {
		t.Fatal("host missing after unlock")
	}

	env := readEnvelope(t, path)

	if env.Format != fileFormat || env.Version != fileVersion || env.Password.Version != fileVersion {
		t.Fatalf("not upgraded: %s v%d, password v%d", env.Format, env.Version, env.Password.Version)
	}

	again, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if err := again.Unlock(testPassword); err != nil {
		t.Fatal(err)
	}

	if _, ok := again.Host("h1"); !ok {
		t.Fatal("host missing after reopening")
	}
}

func TestLegacyVaultWithOSKeyUpgrades(t *testing.T) {
	path, _ := writeLegacy(t, true)
	v, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if v.Locked() {
		t.Fatal("expected the OS key to unlock the vault")
	}

	env := readEnvelope(t, path)

	if env.Format != fileFormat {
		t.Fatalf("format not upgraded: %s", env.Format)
	}

	if env.Password.Version != 0 {
		t.Fatal("the password wrap can't be upgraded without the password")
	}

	again, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	again.mu.Lock()
	again.locked = true
	again.mu.Unlock()

	if err := again.Unlock(testPassword); err != nil {
		t.Fatal(err)
	}

	if readEnvelope(t, path).Password.Version != fileVersion {
		t.Fatal("password wrap not upgraded")
	}
}

func TestNewerVaultIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), vaultFile)
	raw, _ := json.Marshal(envelope{Format: fileFormat, Version: fileVersion + 1})

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenAt(path); err != ErrNewerVersion {
		t.Fatalf("got %v, want ErrNewerVersion", err)
	}
}
