package keyring

import (
	"errors"
	"sync"
)

var (
	ErrUnsupported = errors.New("keyring: no system keyring is available")
	ErrNotFound    = errors.New("keyring: secret not found")
	ErrDismissed   = errors.New("keyring: the request was dismissed")
)

type backend struct {
	set func(id, label string, secret []byte) error
	get func(id string) ([]byte, error)
	del func(id string) error
}

var (
	mu     sync.Mutex
	active = backend{set: systemSet, get: systemGet, del: systemDelete}
)

func Set(id, label string, secret []byte) error {
	return current().set(id, label, secret)
}

func Get(id string) ([]byte, error) {
	return current().get(id)
}

func Delete(id string) error {
	return current().del(id)
}

func UseMemory() {
	store := map[string][]byte{}
	var lock sync.Mutex

	mu.Lock()
	defer mu.Unlock()

	active = backend{
		set: func(id, label string, secret []byte) error {
			lock.Lock()
			defer lock.Unlock()
			store[id] = append([]byte(nil), secret...)
			return nil
		},
		get: func(id string) ([]byte, error) {
			lock.Lock()
			defer lock.Unlock()

			if v, ok := store[id]; ok {
				return append([]byte(nil), v...), nil
			}

			return nil, ErrNotFound
		},
		del: func(id string) error {
			lock.Lock()
			defer lock.Unlock()
			delete(store, id)
			return nil
		},
	}
}

func current() backend {
	mu.Lock()
	defer mu.Unlock()

	return active
}
