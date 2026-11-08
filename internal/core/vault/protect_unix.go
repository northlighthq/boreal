//go:build !windows

package vault

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"

	"github.com/northlighthq/boreal/internal/platform/keyring"
)

var keyringRef = []byte("keyring:")

func protect(plain []byte) ([]byte, error) {
	id := make([]byte, 16)
	rand.Read(id)
	name := hex.EncodeToString(id)

	if err := keyring.Set(name, "Boreal vault key", plain); err != nil {
		return nil, ErrNoKeyring
	}

	return append(bytes.Clone(keyringRef), name...), nil
}

func unprotect(enc []byte) ([]byte, error) {
	if id, ok := bytes.CutPrefix(enc, keyringRef); ok {
		return keyring.Get(string(id))
	}

	return bytes.Clone(enc), nil
}

func plainOS(enc []byte) bool {
	return len(enc) > 0 && !bytes.HasPrefix(enc, keyringRef)
}

func forget(enc []byte) {
	if id, ok := bytes.CutPrefix(enc, keyringRef); ok {
		keyring.Delete(string(id))
	}
}
