//go:build linux && !android

package sessionlock

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	login1      = "org.freedesktop.login1"
	sessionPath = "/org/freedesktop/login1/session/auto"
	lockedHint  = "org.freedesktop.login1.Session.LockedHint"
)

var (
	mu   sync.Mutex
	conn *dbus.Conn
)

func Locked() bool {
	mu.Lock()
	defer mu.Unlock()

	if conn == nil {
		c, err := dbus.ConnectSystemBus()

		if err != nil {
			return false
		}

		conn = c
	}

	v, err := conn.Object(login1, sessionPath).GetProperty(lockedHint)

	if err != nil {
		conn.Close()
		conn = nil
		return false
	}

	locked, _ := v.Value().(bool)
	return locked
}
