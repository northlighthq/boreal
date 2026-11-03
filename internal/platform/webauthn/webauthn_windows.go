package webauthn

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clientDataVersion   = 1
	credentialVersion   = 1
	assertOptionsVer    = 1
	assertTimeoutMillis = 60_000
	attachmentAny       = 0
	verifyRequired      = 1
	verifyDiscouraged   = 3
	hashSHA256          = "SHA-256"
	credentialType      = "public-key"
)

var (
	webauthnDLL       = windows.NewLazySystemDLL("webauthn.dll")
	user32            = windows.NewLazySystemDLL("user32.dll")
	procGetAssertion  = webauthnDLL.NewProc("WebAuthNAuthenticatorGetAssertion")
	procFreeAssertion = webauthnDLL.NewProc("WebAuthNFreeAssertion")
	procErrorName     = webauthnDLL.NewProc("WebAuthNGetErrorName")
	procForeground    = user32.NewProc("GetForegroundWindow")
)

type clientData struct {
	version uint32
	size    uint32
	data    *byte
	hashAlg *uint16
}

type credential struct {
	version uint32
	size    uint32
	id      *byte
	kind    *uint16
}

type credentials struct {
	count uint32
	list  *credential
}

type extensions struct {
	count uint32
	list  uintptr
}

type assertOptions struct {
	version    uint32
	timeout    uint32
	creds      credentials
	exts       extensions
	attachment uint32
	verify     uint32
	flags      uint32
}

type assertion struct {
	version  uint32
	authSize uint32
	auth     *byte
	sigSize  uint32
	sig      *byte
}

func Assert(r Request) (Assertion, error) {
	if procGetAssertion.Find() != nil || len(r.ClientData) == 0 || len(r.Credential) == 0 {
		return Assertion{}, ErrUnsupported
	}

	rp, err := windows.UTF16PtrFromString(r.RPID)

	if err != nil {
		return Assertion{}, err
	}

	hash, _ := windows.UTF16PtrFromString(hashSHA256)
	kind, _ := windows.UTF16PtrFromString(credentialType)

	cd := clientData{version: clientDataVersion, size: uint32(len(r.ClientData)), data: &r.ClientData[0], hashAlg: hash}
	cred := credential{version: credentialVersion, size: uint32(len(r.Credential)), id: &r.Credential[0], kind: kind}
	opts := assertOptions{version: assertOptionsVer, timeout: assertTimeoutMillis, creds: credentials{count: 1, list: &cred}, attachment: attachmentAny, verify: verifyDiscouraged}

	if r.Verify {
		opts.verify = verifyRequired
	}

	hwnd, _, _ := procForeground.Call()
	var out *assertion
	ret, _, _ := procGetAssertion.Call(hwnd, uintptr(unsafe.Pointer(rp)), uintptr(unsafe.Pointer(&cd)), uintptr(unsafe.Pointer(&opts)), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(r)
	runtime.KeepAlive(&cred)
	hr := uint32(ret)

	if hr != 0 || out == nil {
		return Assertion{}, failure(hr)
	}

	defer procFreeAssertion.Call(uintptr(unsafe.Pointer(out)))

	return Assertion{
		AuthData:  append([]byte(nil), unsafe.Slice(out.auth, out.authSize)...),
		Signature: append([]byte(nil), unsafe.Slice(out.sig, out.sigSize)...),
	}, nil
}

func failure(hr uint32) error {
	name, _, _ := procErrorName.Call(uintptr(hr))

	if name == 0 {
		return windows.Errno(hr)
	}

	return errors.New("security key: " + windows.UTF16PtrToString((*uint16)(unsafe.Add(nil, name))))
}
