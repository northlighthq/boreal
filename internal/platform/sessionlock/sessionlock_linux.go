//go:build linux && !android

package sessionlock

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	Supported       = true
	login1          = "org.freedesktop.login1"
	sessionPath     = "/org/freedesktop/login1/session/auto"
	lockedHint      = "org.freedesktop.login1.Session.LockedHint"
	screenSaver     = "org.freedesktop.ScreenSaver"
	screenSaverPath = "/org/freedesktop/ScreenSaver"
	getActive       = screenSaver + ".GetActive"
)

var (
	mu         sync.Mutex
	systemBus  *dbus.Conn
	sessionBus *dbus.Conn
)

func Locked() bool {
	mu.Lock()
	defer mu.Unlock()

	if locked, ok := logindLocked(); ok {
		return locked
	}

	return screenSaverActive()
}

func logindLocked() (bool, bool) {
	if systemBus == nil {
		bus, err := dbus.ConnectSystemBus()

		if err != nil {
			return false, false
		}

		systemBus = bus
	}

	hint, err := systemBus.Object(login1, sessionPath).GetProperty(lockedHint)

	if err != nil {
		systemBus.Close()
		systemBus = nil
		return false, false
	}

	locked, ok := hint.Value().(bool)
	return locked, ok
}

func screenSaverActive() bool {
	if sessionBus == nil {
		bus, err := dbus.ConnectSessionBus()

		if err != nil {
			return false
		}

		sessionBus = bus
	}

	var active bool

	if err := sessionBus.Object(screenSaver, screenSaverPath).Call(getActive, 0).Store(&active); err != nil {
		sessionBus.Close()
		sessionBus = nil
		return false
	}

	return active
}
