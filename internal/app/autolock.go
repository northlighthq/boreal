package app

import (
	"sync/atomic"
	"time"

	"github.com/northlighthq/boreal/internal/platform/hello"
	"github.com/northlighthq/boreal/internal/platform/sessionlock"
)

const (
	sessionPollEvery = 2 * time.Second
	helloMessage     = "Unlock Boreal"
)

// autoLock watches the Windows session and remembers whether Windows Hello
// can unlock the vault.
type autoLock struct {
	helloReady    atomic.Bool
	sessionLocked atomic.Bool
	pending       atomic.Bool
}

func (a *App) startAutoLock() {
	go func() {
		a.autoLock.helloReady.Store(hello.Available())
		a.Invalidate()
	}()

	go a.watchSession()
}

// watchSession notices when Windows locks the session and asks the next
// frame to lock the vault.
func (a *App) watchSession() {
	t := time.NewTicker(sessionPollEvery)
	defer t.Stop()

	for range t.C {
		locked := sessionlock.Locked()

		if locked == a.autoLock.sessionLocked.Swap(locked) || !locked {
			continue
		}

		a.autoLock.pending.Store(true)
		a.Invalidate()
	}
}

func (a *App) systemLock() {
	if !a.autoLock.pending.Swap(false) || !a.settings.LockOnSystemLock || !a.CanLock() {
		return
	}

	a.LockVault()
}

// canHello tells whether Windows Hello can open the vault: it needs Hello to
// be set up and the vault key to be kept by Windows.
func (a *App) canHello() bool {
	return a.autoLock.helloReady.Load() && a.vault.OSUnlock()
}

// CanLock tells whether the vault can be locked and opened again: with the
// master password or with Windows Hello.
func (a *App) CanLock() bool {
	return a.vault.HasPassword() || a.canHello()
}

func (a *App) unlockWithHello() {
	l := &a.lock

	if l.busy {
		return
	}

	l.busy, l.err = true, ""
	l.result = make(chan error, 1)
	result, hwnd := l.result, a.view

	go func() {
		err := hello.Verify(hwnd, helloMessage)

		if err == nil {
			err = a.vault.UnlockOS()
		}

		result <- err
		a.Invalidate()
	}()
}
