package sessionlock

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	currentServer    = 0
	currentSession   = 0xffffffff
	wtsSessionInfoEx = 25
	sessionLocked    = 0

	// Level, padding, SessionId, SessionState
	flagsOffset = 16
	infoSize    = flagsOffset + 4
)

var (
	wtsapi               = windows.NewLazySystemDLL("wtsapi32.dll")
	procQuerySessionInfo = wtsapi.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory    = wtsapi.NewProc("WTSFreeMemory")
)

func Locked() bool {
	var buf *byte
	var size uint32

	ok, _, _ := procQuerySessionInfo.Call(currentServer, currentSession, wtsSessionInfoEx, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&size)))

	if ok == 0 || buf == nil {
		return false
	}

	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(buf)))

	if size < infoSize {
		return false
	}

	info := unsafe.Slice(buf, size)
	flags := int32(info[flagsOffset]) | int32(info[flagsOffset+1])<<8 | int32(info[flagsOffset+2])<<16 | int32(info[flagsOffset+3])<<24
	return flags == sessionLocked
}
