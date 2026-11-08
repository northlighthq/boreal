package vault

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	OSOwner = "Windows user"
	OSName  = "Windows"
)

var (
	entropy       = []byte("boreal-vault-v3")
	legacyEntropy = []byte("platty-vault-v1")
)

func protect(plain []byte) ([]byte, error) {
	return dpapi(plain, entropy, true)
}

func unprotect(enc []byte) ([]byte, error) {
	plain, err := dpapi(enc, entropy, false)

	if err == nil {
		return plain, nil
	}

	return dpapi(enc, legacyEntropy, false)
}

func dpapi(in, entropy []byte, encrypt bool) ([]byte, error) {
	if len(in) == 0 {
		return nil, nil
	}

	inBlob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	entBlob := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var out windows.DataBlob

	var err error

	if encrypt {
		err = windows.CryptProtectData(&inBlob, nil, &entBlob, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&inBlob, nil, &entBlob, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}

	if err != nil {
		return nil, err
	}

	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))

	res := make([]byte, out.Size)
	copy(res, unsafe.Slice(out.Data, out.Size))
	return res, nil
}

func plainOS(enc []byte) bool { return false }

func forget(enc []byte) {}
