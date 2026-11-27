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

func device(bus *dbus.Conn) (dbus.BusObject, bool) {
	var path dbus.ObjectPath

	if bus.Object(fprint, managerPath).Call(managerIface+".GetDefaultDevice", 0).Store(&path) != nil {
		return nil, false
	}

	return bus.Object(fprint, path), true
}

func Available() bool {
	bus, err := dbus.ConnectSystemBus()

	if err != nil {
		return false
	}

	defer bus.Close()

	reader, ok := device(bus)

	if !ok {
		return false
	}

	var fingers []string
	return reader.Call(deviceIface+".ListEnrolledFingers", 0, "").Store(&fingers) == nil && len(fingers) > 0
}

func Verify(hwnd uintptr, message string) error {
	bus, err := dbus.ConnectSystemBus()

	if err != nil {
		return ErrUnavailable
	}

	defer bus.Close()

	reader, ok := device(bus)

	if !ok {
		return ErrUnavailable
	}

	if reader.Call(deviceIface+".Claim", 0, "").Err != nil {
		return ErrUnavailable
	}

	defer reader.Call(deviceIface+".Release", 0)

	if err := bus.AddMatchSignal(dbus.WithMatchObjectPath(reader.Path()), dbus.WithMatchInterface(deviceIface), dbus.WithMatchMember(verifyStatus)); err != nil {
		return ErrUnavailable
	}

	signals := make(chan *dbus.Signal, 8)
	bus.Signal(signals)

	if reader.Call(deviceIface+".VerifyStart", 0, anyFinger).Err != nil {
		return ErrUnavailable
	}

	defer reader.Call(deviceIface+".VerifyStop", 0)

	timeout := time.After(verifyTimeout)

	for {
		select {
		case <-timeout:
			return ErrCanceled
		case signal := <-signals:
			if signal == nil || signal.Name != deviceIface+"."+verifyStatus || len(signal.Body) < 2 {
				continue
			}

			result, _ := signal.Body[0].(string)
			done, _ := signal.Body[1].(bool)

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
