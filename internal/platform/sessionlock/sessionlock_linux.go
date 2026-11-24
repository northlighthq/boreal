//go:build linux && !android

package sessionlock

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	Supported      = true
	logindName     = "org.freedesktop.login1"
	logindSession  = "/org/freedesktop/login1/session/auto"
	lockedHint     = "org.freedesktop.login1.Session.LockedHint"
	screenSaverObj = "/org/freedesktop/ScreenSaver"
	screenSaver    = "org.freedesktop.ScreenSaver"
)

var (
	mu      sync.Mutex
	system  *dbus.Conn
	session *dbus.Conn
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
	if system == nil {
		conn, err := dbus.ConnectSystemBus()

		if err != nil {
			return false, false
		}

		system = conn
	}

	v, err := system.Object(logindName, logindSession).GetProperty(lockedHint)

	if err != nil {
		return false, false
	}

	locked, ok := v.Value().(bool)
	return locked, ok
}

func screenSaverActive() bool {
	if session == nil {
		conn, err := dbus.ConnectSessionBus()

		if err != nil {
			return false
		}

		session = conn
	}

	var active bool

	if session.Object(screenSaver, screenSaverObj).Call(screenSaver+".GetActive", 0).Store(&active) != nil {
		return false
	}

	return active
}
