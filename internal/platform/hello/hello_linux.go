//go:build linux && !android

package hello

import (
	"time"

	"github.com/godbus/dbus/v5"
)

const Name = "Fingerprint"

const (
	fprint        = "net.reactivated.Fprint"
	managerPath   = "/net/reactivated/Fprint/Manager"
	managerIface  = fprint + ".Manager"
	deviceIface   = fprint + ".Device"
	verifyTimeout = 30 * time.Second
	matched       = "verify-match"
	noMatch       = "verify-no-match"
	verifyStatus  = "VerifyStatus"
	anyFinger     = "any"
)

func device(conn *dbus.Conn) (dbus.BusObject, bool) {
	var path dbus.ObjectPath

	if conn.Object(fprint, managerPath).Call(managerIface+".GetDefaultDevice", 0).Store(&path) != nil {
		return nil, false
	}

	return conn.Object(fprint, path), true
}

func Available() bool {
	conn, err := dbus.ConnectSystemBus()

	if err != nil {
		return false
	}

	defer conn.Close()

	dev, ok := device(conn)

	if !ok {
		return false
	}

	var fingers []string
	return dev.Call(deviceIface+".ListEnrolledFingers", 0, "").Store(&fingers) == nil && len(fingers) > 0
}

func Verify(hwnd uintptr, message string) error {
	conn, err := dbus.ConnectSystemBus()

	if err != nil {
		return ErrUnavailable
	}

	defer conn.Close()

	dev, ok := device(conn)

	if !ok {
		return ErrUnavailable
	}

	if dev.Call(deviceIface+".Claim", 0, "").Err != nil {
		return ErrUnavailable
	}

	defer dev.Call(deviceIface+".Release", 0)

	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(dev.Path()), dbus.WithMatchInterface(deviceIface), dbus.WithMatchMember(verifyStatus)); err != nil {
		return ErrUnavailable
	}

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)

	if dev.Call(deviceIface+".VerifyStart", 0, anyFinger).Err != nil {
		return ErrUnavailable
	}

	defer dev.Call(deviceIface+".VerifyStop", 0)

	timeout := time.After(verifyTimeout)

	for {
		select {
		case <-timeout:
			return ErrCanceled
		case s := <-signals:
			if s == nil || s.Name != deviceIface+"."+verifyStatus || len(s.Body) < 2 {
				continue
			}

			result, _ := s.Body[0].(string)
			done, _ := s.Body[1].(bool)

			switch {
			case result == matched:
				return nil
			case done && result == noMatch:
				return ErrFailed
			case done:
				return ErrCanceled
			}
		}
	}
}
