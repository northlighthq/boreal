package backup

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/northlighthq/boreal/internal/secure"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	Extension   = ".platty"
	MinPassword = 12

	fileFormat  = "platty-export"
	fileVersion = 1

	kdfTime   = 4
	kdfMemory = 128 * 1024
)

var (
	ErrNotExport     = errors.New("this isn't a Boreal export file")
	ErrNewerVersion  = errors.New("this export was made by a newer version of Boreal")
	ErrWrongPassword = errors.New("wrong password, or the file is damaged")
	ErrShortPassword = errors.New("the password must be at least 12 characters long")

	aad = []byte("platty-export-v1")
)

type Selection struct {
	Hosts      bool
	Keys       bool
	Secrets    bool
	Forwards   bool
	Snippets   bool
	KnownHosts bool
}

type file struct {
	Format  string     `json:"format"`
	Version int        `json:"version"`
	Created time.Time  `json:"created"`
	KDF     secure.KDF `json:"kdf"`
	Data    []byte     `json:"data"`
}

func Filter(d vault.Data, s Selection) vault.Data {
	out := vault.Data{Version: d.Version}

	if s.Hosts {
		out.Hosts = d.Hosts
		out.Folders = d.Folders
	}

	if s.Keys {
		out.Keys = d.Keys
	}

	if s.Hosts && s.Forwards {
		out.Forwards = d.Forwards
	}

	if s.Snippets {
		out.Snippets = d.Snippets
	}

	if s.KnownHosts {
		out.KnownHosts = d.KnownHosts
	}

	if s.Secrets {
		return out
	}

	hosts := make([]vault.Host, len(out.Hosts))

	for i, h := range out.Hosts {
		h.Password = ""
		hosts[i] = h
	}

	keys := make([]vault.Key, len(out.Keys))

	for i, k := range out.Keys {
		k.Passphrase = ""
		keys[i] = k
	}

	out.Hosts, out.Keys = hosts, keys
	return out
}

func Export(d vault.Data, s Selection, password string) ([]byte, error) {
	if len([]rune(password)) < MinPassword {
		return nil, ErrShortPassword
	}

	plain, err := json.Marshal(Filter(d, s))

	if err != nil {
		return nil, err
	}

	defer secure.Wipe(plain)

	kdf := secure.NewKDF(kdfTime, kdfMemory)
	key, err := kdf.Key(password)

	if err != nil {
		return nil, err
	}

	defer secure.Wipe(key)

	sealed, err := secure.Seal(key, plain, aad)

	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(file{Format: fileFormat, Version: fileVersion, Created: time.Now().UTC(), KDF: kdf, Data: sealed}, "", "  ")
}

func Import(raw []byte, password string) (vault.Data, error) {
	var f file

	if json.Unmarshal(raw, &f) != nil || f.Format != fileFormat {
		return vault.Data{}, ErrNotExport
	}

	if f.Version > fileVersion {
		return vault.Data{}, ErrNewerVersion
	}

	key, err := f.KDF.Key(password)

	if err != nil {
		return vault.Data{}, err
	}

	defer secure.Wipe(key)

	plain, err := secure.Open(key, f.Data, aad)

	if err != nil {
		return vault.Data{}, ErrWrongPassword
	}

	defer secure.Wipe(plain)

	var d vault.Data

	if err := json.Unmarshal(plain, &d); err != nil {
		return vault.Data{}, ErrNotExport
	}

	return d, nil
}

func GeneratePassword() string {
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secure.Random(15))
	s = strings.ToLower(s)

	var groups []string

	for len(s) > 0 {
		n := min(6, len(s))
		groups = append(groups, s[:n])
		s = s[n:]
	}

	return strings.Join(groups, "-")
}
