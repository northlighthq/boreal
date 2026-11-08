package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"runtime/debug"

	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/secure"
)

const (
	fileFormat   = "boreal-vault"
	fileVersion  = 3
	legacyFormat = "platty-vault"

	vaultKDFTime   = 3
	vaultKDFMemory = 64 * 1024
)

var (
	ErrLocked        = errors.New("vault: locked")
	ErrWrongPassword = errors.New("vault: wrong password")
	ErrNoPassword    = errors.New("vault: no master password set")
	ErrNeedUnlock    = errors.New("vault: it can't be unlocked automatically on this PC")
	ErrNewerVersion  = errors.New("vault: it was saved by a newer version of Boreal")
	ErrNoKeyring     = errors.New("vault: no system keyring is available to protect the vault")
	ErrNeedPassword  = errors.New("vault: set a master password before saving")

	dataAAD       = []byte("boreal-vault-v3")
	keyAAD        = []byte("boreal-vault-key-v3")
	legacyDataAAD = []byte("platty-vault-v2")
	legacyKeyAAD  = []byte("platty-vault-key")
)

type envelope struct {
	Format   string   `json:"format"`
	Version  int      `json:"version"`
	OS       []byte   `json:"os,omitempty"`
	Password *wrapped `json:"password,omitempty"`
	Data     []byte   `json:"data,omitempty"`
}

type wrapped struct {
	KDF     secure.KDF `json:"kdf"`
	Key     []byte     `json:"key"`
	Version int        `json:"version,omitempty"`
}

func (e envelope) dataAAD() []byte {
	if e.Format == legacyFormat {
		return legacyDataAAD
	}

	return dataAAD
}

func (w *wrapped) aad() []byte {
	if w.Version < fileVersion {
		return legacyKeyAAD
	}

	return keyAAD
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

	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) || json.Unmarshal(raw, &env) != nil || (env.Format != fileFormat && env.Format != legacyFormat) {
		return v, v.migrate(raw)
	}

	if env.Format == fileFormat && env.Version > fileVersion {
		return nil, ErrNewerVersion
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

	if err := v.unlockWith(key); err != nil {
		return nil, err
	}

	v.reprotect()
	return v, nil
}

func (v *Vault) create() error {
	v.key = secure.NewKey()
	v.env = envelope{Format: fileFormat, Version: fileVersion}

	if osKey, err := protect(v.key); err == nil {
		v.env.OS = osKey
	}

	return nil
}

func (v *Vault) reprotect() {
	if v.env.Password != nil || !plainOS(v.env.OS) {
		return
	}

	osKey, err := protect(v.key)

	if err != nil {
		return
	}

	prev := v.env.OS
	v.env.OS = osKey

	if v.save() != nil {
		forget(osKey)
		v.env.OS = prev
	}
}

func (v *Vault) exposed() bool {
	return v.env.Password == nil && (v.env.OS == nil || plainOS(v.env.OS))
}

func (v *Vault) NeedsPassword() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return !v.locked && v.exposed()
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

	if err := v.save(); err != nil && !errors.Is(err, ErrNeedPassword) {
		return err
	}

	return nil
}

func (v *Vault) unlockWith(key []byte) error {
	var d Data

	if len(v.env.Data) > 0 {
		plain, err := secure.Open(key, v.env.Data, v.env.dataAAD())

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
	v.upgrade()
	return nil
}

func (v *Vault) upgrade() {
	if v.env.Format != legacyFormat {
		return
	}

	prev := v.env
	v.env.Format, v.env.Version = fileFormat, fileVersion

	if prev.OS != nil {
		osKey, err := protect(v.key)

		if err != nil {
			v.env = prev
			return
		}

		v.env.OS = osKey
	}

	if err := v.save(); err != nil {
		if prev.OS != nil {
			forget(v.env.OS)
		}

		v.env = prev
		return
	}

	if prev.OS != nil {
		forget(prev.OS)
	}
}

func (v *Vault) save() error {
	if v.locked {
		return ErrLocked
	}

	if v.env.OS == nil && v.env.Password == nil {
		return ErrNeedPassword
	}

	plain, err := json.Marshal(v.data)

	if err != nil {
		return err
	}

	sealed, err := secure.Seal(v.key, plain, v.env.dataAAD())
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

	legacy := v.env.Password.Version < fileVersion
	key, err := v.passwordKey(password)

	if err != nil {
		return err
	}

	if err := v.unlockWith(key); err != nil {
		return err
	}

	if !legacy {
		return nil
	}

	return v.save()
}

func (v *Vault) UnlockOS() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.locked {
		return nil
	}

	if v.env.OS == nil {
		return ErrNeedUnlock
	}

	key, err := unprotect(v.env.OS)

	if err != nil {
		return ErrNeedUnlock
	}

	return v.unlockWith(key)
}

func (v *Vault) passwordKey(password string) ([]byte, error) {
	kek, err := v.env.Password.KDF.Key(password)

	if err != nil {
		return nil, err
	}

	defer secure.Wipe(kek)

	w := v.env.Password
	key, err := secure.Open(kek, w.Key, w.aad())

	if err != nil {
		return nil, ErrWrongPassword
	}

	if w.Version >= fileVersion {
		return key, nil
	}

	sealed, err := secure.Seal(kek, key, keyAAD)

	if err != nil {
		return key, nil
	}

	v.env.Password = &wrapped{KDF: w.KDF, Key: sealed, Version: fileVersion}
	return key, nil
}

func (v *Vault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.env.Password == nil && v.env.OS == nil {
		return ErrNoPassword
	}

	if v.locked {
		return nil
	}

	secure.Wipe(v.key)
	v.key, v.data, v.locked = nil, Data{Version: version}, true
	v.rev++
	go debug.FreeOSMemory()
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

	keepOS := v.env.OS != nil && !plainOS(v.env.OS)

	if next == "" {
		return v.rekey(true, "")
	}

	return v.rekey(keepOS, next)
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

	if on == (v.env.OS != nil && !plainOS(v.env.OS)) {
		return nil
	}

	if on {
		osKey, err := protect(v.key)

		if err != nil {
			return err
		}

		old := v.env.OS
		v.env.OS = osKey
		v.rev++

		if err := v.save(); err != nil {
			forget(osKey)
			v.env.OS = old
			return err
		}

		forget(old)
		return nil
	}

	old := v.env.OS
	v.env.OS = nil
	v.rev++

	if err := v.save(); err != nil {
		v.env.OS = old
		return err
	}

	forget(old)
	return nil
}

func (v *Vault) rekey(withOS bool, password string) error {
	prevEnv, prevKey := v.env, v.key
	key := secure.NewKey()
	var w *wrapped

	if password != "" {
		kdf := secure.NewKDF(vaultKDFTime, vaultKDFMemory)
		kek, err := kdf.Key(password)

		if err != nil {
			return err
		}

		sealed, err := secure.Seal(kek, key, keyAAD)
		secure.Wipe(kek)

		if err != nil {
			return err
		}

		w = &wrapped{KDF: kdf, Key: sealed, Version: fileVersion}
	}

	return v.commit(prevEnv, prevKey, key, withOS, w)
}

func (v *Vault) commit(prevEnv envelope, prevKey, key []byte, withOS bool, w *wrapped) error {
	next := envelope{Format: fileFormat, Version: fileVersion, Password: w}

	if withOS {
		osKey, err := protect(key)

		if err != nil {
			return err
		}

		next.OS = osKey
	}

	if next.OS == nil && next.Password == nil {
		return ErrNoKeyring
	}

	v.env, v.key = next, key
	v.rev++

	if err := v.save(); err != nil {
		if next.OS != nil {
			forget(next.OS)
		}

		v.env, v.key = prevEnv, prevKey
		return err
	}

	if prevEnv.OS != nil {
		forget(prevEnv.OS)
	}

	secure.Wipe(prevKey)
	return nil
}

const MinPassword = 10

var (
	ErrShortPassword = errors.New("use at least 10 characters for the master password")
	ErrMismatch      = errors.New("the two passwords don't match")
)

func CheckPassword(password, repeat string) error {
	if len([]rune(password)) < MinPassword {
		return ErrShortPassword
	}

	if password != repeat {
		return ErrMismatch
	}

	return nil
}
