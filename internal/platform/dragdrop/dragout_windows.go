package dragdrop

import (
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	dropEffectCopy        = 1
	mkLButton             = 0x0001
	sOK                   = 0
	eNoInterface          = 0x80004002
	dragDropSDrop         = 0x00040100
	dragDropSCancel       = 0x00040101
	dragDropSUseDefCursor = 0x00040102
)

var (
	ole32             = windows.NewLazySystemDLL("ole32.dll")
	procOleInitialize = ole32.NewProc("OleInitialize")
	procDoDragDrop    = ole32.NewProc("DoDragDrop")

	iidUnknown    = windows.GUID{Data1: 0x00000000, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidDataObject = windows.GUID{Data1: 0x0000010e, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidDropSource = windows.GUID{Data1: 0x00000121, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

type dropSourceVtbl struct {
	queryInterface    uintptr
	addRef            uintptr
	release           uintptr
	queryContinueDrag uintptr
	giveFeedback      uintptr
}

type dropSource struct {
	vtbl *dropSourceVtbl
}

var (
	sourceVtbl = &dropSourceVtbl{
		queryInterface: syscall.NewCallback(func(this uintptr, riid *windows.GUID, ppv *uintptr) uintptr {
			if *riid != iidUnknown && *riid != iidDropSource {
				*ppv = 0
				return eNoInterface
			}

			*ppv = this
			return sOK
		}),
		addRef:  syscall.NewCallback(func(this uintptr) uintptr { return 1 }),
		release: syscall.NewCallback(func(this uintptr) uintptr { return 1 }),
		queryContinueDrag: syscall.NewCallback(func(this, escape, keys uintptr) uintptr {
			switch {
			case int32(escape) != 0:
				return dragDropSCancel
			case keys&mkLButton == 0:
				return dragDropSDrop
			}

			return sOK
		}),
		giveFeedback: syscall.NewCallback(func(this, effect uintptr) uintptr { return dragDropSUseDefCursor }),
	}

	source = &dropSource{vtbl: sourceVtbl}
)
