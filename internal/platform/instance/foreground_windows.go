package instance

import "golang.org/x/sys/windows"

const asfwAny = ^uintptr(0)

var procAllowSetForeground = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

func allowForeground() {
	procAllowSetForeground.Call(asfwAny)
}
