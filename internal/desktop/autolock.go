package desktop

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

type autoLock struct {
	startLocked   bool
	helloReady    atomic.Bool
	sessionLocked atomic.Bool
	pending       atomic.Bool
}

func (a *App) startAutoLock() {
	if a.settings.WindowsHello && a.vault.OSUnlock() && !a.vault.Locked() && a.vault.Lock() == nil {
		a.autoLock.startLocked = true
	}

	startLocked := a.autoLock.startLocked

	go func() {
		ready := hello.Available()
		a.autoLock.helloReady.Store(ready)

		if !ready && startLocked && !a.vault.HasPassword() {
			a.vault.UnlockOS()
		}

		a.Invalidate()
	}()

	go a.watchSession()
}

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

func (a *App) HelloAvailable() bool {
	return a.autoLock.helloReady.Load()
}

func (a *App) canHello() bool {
	return a.settings.WindowsHello && a.HelloAvailable() && a.vault.OSUnlock()
}

func (a *App) CanLock() bool {
	return a.vault.HasPassword() || a.canHello()
}

func (a *App) unlockWithHello() {
	l := &a.lock

	if l.busy {
		return
	}

	l.busy, l.viaHello, l.err = true, true, ""
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
