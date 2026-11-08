package vault

import (
	"path/filepath"
	"testing"
)

func TestLockWithoutPasswordAndUnlockOS(t *testing.T) {
	v, err := OpenAt(filepath.Join(t.TempDir(), vaultFile))

	if err != nil {
		t.Fatal(err)
	}

	if _, err := v.SaveHost(Host{Address: "example.com", Password: "secret"}); err != nil {
		t.Fatal(err)
	}

	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}

	if !v.Locked() || len(v.Snapshot().Hosts) != 0 {
		t.Fatal("locking must hide the data")
	}

	if err := v.Unlock("anything"); err != ErrNoPassword {
		t.Fatalf("got %v, want ErrNoPassword", err)
	}

	if err := v.UnlockOS(); err != nil {
		t.Fatal(err)
	}

	hosts := v.Snapshot().Hosts

	if len(hosts) != 1 || hosts[0].Password != "secret" {
		t.Fatalf("unexpected hosts after unlock: %+v", hosts)
	}
}

func TestUnlockOSNeedsOSKey(t *testing.T) {
	v, err := OpenAt(filepath.Join(t.TempDir(), vaultFile))

	if err != nil {
		t.Fatal(err)
	}

	if err := v.SetPassword("", "a long master password"); err != nil {
		t.Fatal(err)
	}

	if err := v.SetOSUnlock(false); err != nil {
		t.Fatal(err)
	}

	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}

	if err := v.UnlockOS(); err != ErrNeedUnlock {
		t.Fatalf("got %v, want ErrNeedUnlock", err)
	}
}

func TestSetTriggers(t *testing.T) {
	path := filepath.Join(t.TempDir(), vaultFile)
	v, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if err := v.SetTriggers([]Trigger{{Pattern: "error", Color: TriggerRed, Notify: true}}); err != nil {
		t.Fatal(err)
	}

	again, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	got := again.Snapshot().Triggers

	if len(got) != 1 || got[0].ID == "" || got[0].Pattern != "error" || !got[0].Notify {
		t.Fatalf("triggers should be saved with an ID, got %+v", got)
	}

	if _, err := (Trigger{Pattern: " "}).Compile(); err != ErrEmptyPattern {
		t.Fatalf("an empty pattern should not compile, got %v", err)
	}
}
