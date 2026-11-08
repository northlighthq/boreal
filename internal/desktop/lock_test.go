package desktop

import (
	"path/filepath"
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestWindowsHelloSetting(t *testing.T) {
	v, err := vault.OpenAt(filepath.Join(t.TempDir(), "vault.dat"))

	if err != nil {
		t.Fatal(err)
	}

	if !v.OSUnlock() {
		t.Skip("no system key store here")
	}

	a := &App{vault: v}
	a.autoLock.helloReady.Store(true)
	a.settings.WindowsHello = true

	if !a.canHello() || !a.CanLock() {
		t.Fatal("Windows Hello should unlock and allow locking when it's on")
	}

	a.settings.WindowsHello = false

	if a.canHello() {
		t.Fatal("Windows Hello should not be offered when it's off")
	}

	if a.CanLock() {
		t.Fatal("without Windows Hello or a password, Boreal must not lock: it couldn't open again")
	}

	if !v.OSUnlock() {
		t.Fatal("turning Windows Hello off must not change how the vault is stored")
	}
}
