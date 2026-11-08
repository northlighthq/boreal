package secure

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	KeySize  = chacha20poly1305.KeySize
	saltSize = 16

	maxTime    = 8
	maxMemory  = 256 * 1024
	maxThreads = 8
)

var (
	ErrDecrypt   = errors.New("secure: wrong password or damaged data")
	ErrBadParams = errors.New("secure: invalid key derivation parameters")
)

type KDF struct {
	Alg     string `json:"alg"`
	Salt    []byte `json:"salt"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
}

func NewKDF(time, memoryKiB uint32) KDF {
	return KDF{Alg: "argon2id", Salt: Random(saltSize), Time: time, Memory: memoryKiB, Threads: 4}
}

func (k KDF) Key(password string) ([]byte, error) {
	if k.Alg != "argon2id" || len(k.Salt) < saltSize || k.Time == 0 || k.Time > maxTime || k.Memory < 8*1024 || k.Memory > maxMemory || k.Threads == 0 || k.Threads > maxThreads {
		return nil, ErrBadParams
	}

	return argon2.IDKey([]byte(password), k.Salt, k.Time, k.Memory, k.Threads, KeySize), nil
}

func Random(n int) []byte {
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("secure: no randomness: %v", err))
	}

	return b
}

func NewKey() []byte {
	return Random(KeySize)
}

func Seal(key, plain, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)

	if err != nil {
		return nil, err
	}

	nonce := Random(aead.NonceSize())
	return aead.Seal(nonce, nonce, plain, aad), nil
}

func Open(key, sealed, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)

	if err != nil {
		return nil, err
	}

	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrDecrypt
	}

	nonce, body := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, body, aad)

	if err != nil {
		return nil, ErrDecrypt
	}

	return plain, nil
}

func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
