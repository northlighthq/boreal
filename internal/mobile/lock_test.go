package mobile

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestLockIfAway(t *testing.T) {
	v, err := vault.OpenAt(filepath.Join(t.TempDir(), vault.FileName))

	if err != nil {
		t.Fatal(err)
	}

	a := &App{vault: v, settings: config.Settings{LockAfter: 5}}
	now := time.Unix(10_000, 0)
	a.hiddenAt = now.Add(-10 * time.Minute)
	a.lockIfAway(now)

	if v.Locked() {
		t.Fatal("a vault without a master password can't lock")
	}

	if err := v.SetPassword("", "correct horse battery"); err != nil {
		t.Fatal(err)
	}

	a.hiddenAt = now.Add(-time.Minute)
	a.lockIfAway(now)

	if v.Locked() {
		t.Fatal("a short trip to the background must not lock")
	}

	a.settings.LockAfter = 0
	a.hiddenAt = now.Add(-time.Hour)
	a.lockIfAway(now)

	if v.Locked() {
		t.Fatal("Never must not lock")
	}

	a.settings.LockAfter = 5
	a.lockIfAway(now)

	if !v.Locked() {
		t.Fatal("coming back after the limit must lock the vault")
	}
}
