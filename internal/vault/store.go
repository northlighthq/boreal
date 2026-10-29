package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"

	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/secure"
)

const (
	fileFormat  = "platty-vault"
	fileVersion = 2

	vaultKDFTime   = 3
	vaultKDFMemory = 64 * 1024
)

var (
	ErrLocked        = errors.New("vault: locked")
	ErrWrongPassword = errors.New("vault: wrong password")
	ErrNoPassword    = errors.New("vault: no master password set")
	ErrNeedUnlock    = errors.New("vault: it can't be unlocked automatically on this PC")

	dataAAD = []byte("platty-vault-v2")
	keyAAD  = []byte("platty-vault-key")
)

type envelope struct {
	Format   string   `json:"format"`
	Version  int      `json:"version"`
	OS       []byte   `json:"os,omitempty"`
	Password *wrapped `json:"password,omitempty"`
	Data     []byte   `json:"data,omitempty"`
}

type wrapped struct {
	KDF secure.KDF `json:"kdf"`
	Key []byte     `json:"key"`
}

func OpenAt(path string) (*Vault, error) {
	v := &Vault{path: path, data: Data{Version: version}}
	raw, err := os.ReadFile(path)

	if errors.Is(err, fs.ErrNotExist) {
		return v, v.create()
	}

	if err != nil {
		return nil, err
	}

	var env envelope

	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) || json.Unmarshal(raw, &env) != nil || env.Format != fileFormat {
		return v, v.migrate(raw)
	}

	v.env = env

	if env.OS == nil {
		v.locked = true
		return v, nil
	}

	key, err := unprotect(env.OS)

	if err != nil {
		if env.Password == nil {
			return nil, ErrNeedUnlock
		}

		v.locked = true
		return v, nil
	}

	return v, v.unlockWith(key)
}

func (v *Vault) create() error {
	v.key = secure.NewKey()
	osKey, err := protect(v.key)

	if err != nil {
		return err
	}

	v.env = envelope{Format: fileFormat, Version: fileVersion, OS: osKey}
	return nil
}

func (v *Vault) migrate(old []byte) error {
	plain, err := unprotect(old)

	if err != nil {
		return err
	}

	if err := json.Unmarshal(plain, &v.data); err != nil {
		return err
	}

	secure.Wipe(plain)

	if err := v.create(); err != nil {
		return err
	}

	return v.save()
}

func (v *Vault) unlockWith(key []byte) error {
	var d Data

	if len(v.env.Data) > 0 {
		plain, err := secure.Open(key, v.env.Data, dataAAD)

		if err != nil {
			return err
		}

		err = json.Unmarshal(plain, &d)
		secure.Wipe(plain)

		if err != nil {
			return err
		}
	}

	if d.Version == 0 {
		d.Version = version
	}

	v.key, v.data, v.locked = key, d, false
	v.rev++
	return nil
}

func (v *Vault) save() error {
	if v.locked {
		return ErrLocked
	}

	plain, err := json.Marshal(v.data)

	if err != nil {
		return err
	}

	sealed, err := secure.Seal(v.key, plain, dataAAD)
	secure.Wipe(plain)

	if err != nil {
		return err
	}

	v.env.Data = sealed
	raw, err := json.Marshal(v.env)

	if err != nil {
		return err
	}

	return config.WriteAtomic(v.path, raw)
}

func (v *Vault) Locked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.locked
}

func (v *Vault) HasPassword() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.env.Password != nil
}

func (v *Vault) OSUnlock() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.env.OS != nil
}

func (v *Vault) Unlock(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.locked {
		return nil
	}

	if v.env.Password == nil {
		return ErrNoPassword
	}

	key, err := v.passwordKey(password)

	if err != nil {
		return err
	}

	return v.unlockWith(key)
}

func (v *Vault) passwordKey(password string) ([]byte, error) {
	kek, err := v.env.Password.KDF.Key(password)

	if err != nil {
		return nil, err
	}

	defer secure.Wipe(kek)

	key, err := secure.Open(kek, v.env.Password.Key, keyAAD)

	if err != nil {
		return nil, ErrWrongPassword
	}

	return key, nil
}

func (v *Vault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.env.Password == nil {
		return ErrNoPassword
	}

	if v.locked {
		return nil
	}

	secure.Wipe(v.key)
	v.key, v.data, v.locked = nil, Data{Version: version}, true
	v.rev++
	return nil
}

func (v *Vault) SetPassword(current, next string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.locked {
		return ErrLocked
	}

	if v.env.Password != nil {
		key, err := v.passwordKey(current)

		if err != nil {
			return err
		}

		secure.Wipe(key)
	}

	if next == "" {
		if v.env.OS == nil {
			osKey, err := protect(v.key)

			if err != nil {
				return err
			}

			v.env.OS = osKey
		}

		v.env.Password = nil
		v.rev++
		return v.save()
	}

	kdf := secure.NewKDF(vaultKDFTime, vaultKDFMemory)
	kek, err := kdf.Key(next)

	if err != nil {
		return err
	}

	defer secure.Wipe(kek)

	sealed, err := secure.Seal(kek, v.key, keyAAD)

	if err != nil {
		return err
	}

	v.env.Password = &wrapped{KDF: kdf, Key: sealed}
	v.rev++
	return v.save()
}

func (v *Vault) SetOSUnlock(on bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.locked {
		return ErrLocked
	}

	if !on && v.env.Password == nil {
		return ErrNoPassword
	}

	if on == (v.env.OS != nil) {
		return nil
	}

	old := v.env.OS
	v.env.OS = nil

	if on {
		osKey, err := protect(v.key)

		if err != nil {
			return err
		}

		v.env.OS = osKey
	}

	v.rev++

	if err := v.save(); err != nil {
		return err
	}

	forget(old)
	return nil
}
