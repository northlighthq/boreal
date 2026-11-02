package hello

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	verifierClass = "Windows.Security.Credentials.UI.UserConsentVerifier"
	resultEnum    = "enum(Windows.Security.Credentials.UI.UserConsentVerificationResult;i4)"
	availableEnum = "enum(Windows.Security.Credentials.UI.UserConsentVerifierAvailability;i4)"
	asyncOpIID    = "{9fc2b0bb-e446-44e2-aa61-9cab8f636af2}"

	roInitMultithreaded = 1
	sFalse              = 1
	rpcChangedMode      = 0x80010106

	asyncStarted   = 0
	asyncCompleted = 1

	statusVerified = 0
	statusCanceled = 6

	pollEvery    = 50 * time.Millisecond
	checkTimeout = 5 * time.Second

	vtQueryInterface    = 0
	vtRelease           = 2
	vtCheckAvailability = 6
	vtRequestForWindow  = 6
	vtAsyncGetStatus    = 7
	vtAsyncOpGetResults = 8
)

var (
	combase                 = windows.NewLazySystemDLL("combase.dll")
	procRoInitialize        = combase.NewProc("RoInitialize")
	procRoUninitialize      = combase.NewProc("RoUninitialize")
	procRoGetActivation     = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString = combase.NewProc("WindowsDeleteString")

	iidStatics   = windows.GUID{Data1: 0xaf4f3f91, Data2: 0x564c, Data3: 0x4ddc, Data4: [8]byte{0xb8, 0xb5, 0x97, 0x34, 0x47, 0x62, 0x7c, 0x65}}
	iidInterop   = windows.GUID{Data1: 0x39e050c3, Data2: 0x4e74, Data3: 0x441a, Data4: [8]byte{0x8d, 0xc0, 0xb8, 0x11, 0x04, 0xdf, 0x94, 0x9c}}
	iidAsyncInfo = windows.GUID{Data1: 0x00000036, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

	winrtNamespace = [16]byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}

	iidAsyncResult    = parameterizedIID("pinterface(" + asyncOpIID + ";" + resultEnum + ")")
	iidAsyncAvailable = parameterizedIID("pinterface(" + asyncOpIID + ";" + availableEnum + ")")

	errTimeout = errors.New("Windows Hello didn't answer in time")
)

func Available() bool {
	status, err := run(func() (int32, error) {
		statics, err := factory(&iidStatics)

		if err != nil {
			return 0, err
		}

		defer release(statics)

		var op *object

		if hr := call(statics, vtCheckAvailability, uintptr(unsafe.Pointer(&op))); failed(hr) {
			return 0, syscall.Errno(hr)
		}

		defer release(op)
		return wait(op, &iidAsyncAvailable, checkTimeout)
	})

	return err == nil && status == statusVerified
}

func Verify(hwnd uintptr, message string) error {
	status, err := run(func() (int32, error) {
		interop, err := factory(&iidInterop)

		if err != nil {
			return 0, err
		}

		defer release(interop)

		msg, err := hstring(message)

		if err != nil {
			return 0, err
		}

		defer procWindowsDeleteString.Call(msg)

		var op *object

		if hr := call(interop, vtRequestForWindow, hwnd, msg, uintptr(unsafe.Pointer(&iidAsyncResult)), uintptr(unsafe.Pointer(&op))); failed(hr) {
			return 0, syscall.Errno(hr)
		}

		defer release(op)
		return wait(op, &iidAsyncResult, 0)
	})

	switch {
	case err != nil:
		return err
	case status == statusVerified:
		return nil
	case status == statusCanceled:
		return ErrCanceled
	case status >= 1 && status <= 3:
		return ErrUnavailable
	default:
		return ErrFailed
	}
}

func run(fn func() (int32, error)) (int32, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procRoInitialize.Call(roInitMultithreaded)

	switch {
	case hr == 0 || hr == sFalse:
		defer procRoUninitialize.Call()
	case uint32(hr) == rpcChangedMode:
	default:
		return 0, syscall.Errno(hr)
	}

	return fn()
}

func factory(iid *windows.GUID) (*object, error) {
	class, err := hstring(verifierClass)

	if err != nil {
		return nil, err
	}

	defer procWindowsDeleteString.Call(class)

	var f *object
	hr, _, _ := procRoGetActivation.Call(class, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f)))

	if failed(hr) {
		return nil, syscall.Errno(hr)
	}

	return f, nil
}

func wait(op *object, iid *windows.GUID, timeout time.Duration) (int32, error) {
	var info *object

	if hr := call(op, vtQueryInterface, uintptr(unsafe.Pointer(&iidAsyncInfo)), uintptr(unsafe.Pointer(&info))); failed(hr) {
		return 0, syscall.Errno(hr)
	}

	defer release(info)

	start := time.Now()

	for {
		var status int32

		if hr := call(info, vtAsyncGetStatus, uintptr(unsafe.Pointer(&status))); failed(hr) {
			return 0, syscall.Errno(hr)
		}

		if status == asyncCompleted {
			break
		}

		if status != asyncStarted {
			return 0, ErrFailed
		}

		if timeout > 0 && time.Since(start) > timeout {
			return 0, errTimeout
		}

		time.Sleep(pollEvery)
	}

	var typed *object

	if hr := call(op, vtQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&typed))); failed(hr) {
		return 0, syscall.Errno(hr)
	}

	defer release(typed)

	var result int32

	if hr := call(typed, vtAsyncOpGetResults, uintptr(unsafe.Pointer(&result))); failed(hr) {
		return 0, syscall.Errno(hr)
	}

	return result, nil
}

type object struct {
	vtbl *[16]uintptr
}

func call(obj *object, method int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(obj.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)...)
	return hr
}

func release(obj *object) {
	if obj != nil {
		call(obj, vtRelease)
	}
}

func failed(hr uintptr) bool {
	return int32(hr) < 0
}

func hstring(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)

	if err != nil {
		return 0, err
	}

	var h uintptr
	hr, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))

	if failed(hr) {
		return 0, syscall.Errno(hr)
	}

	return h, nil
}

func parameterizedIID(signature string) windows.GUID {
	h := sha1.New()
	h.Write(winrtNamespace[:])
	h.Write([]byte(signature))
	sum := h.Sum(nil)

	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80

	g := windows.GUID{
		Data1: binary.BigEndian.Uint32(sum[0:4]),
		Data2: binary.BigEndian.Uint16(sum[4:6]),
		Data3: binary.BigEndian.Uint16(sum[6:8]),
	}

	copy(g.Data4[:], sum[8:16])
	return g
}
