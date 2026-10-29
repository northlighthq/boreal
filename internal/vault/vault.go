package vault

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/northlighthq/boreal/internal/config"
)

const (
	vaultFile = "vault.dat"
	version   = 1
)

const (
	AuthPassword = "password"
	AuthKey      = "key"
	AuthAgent    = "agent"
)

const (
	ForwardLocal   = "local"
	ForwardRemote  = "remote"
	ForwardDynamic = "dynamic"
)

type Host struct {
	ID             string    `json:"id"`
	Label          string    `json:"label"`
	Address        string    `json:"address"`
	Port           int       `json:"port"`
	Username       string    `json:"username"`
	Auth           string    `json:"auth"`
	Password       string    `json:"password,omitempty"`
	KeyID          string    `json:"keyId,omitempty"`
	JumpID         string    `json:"jumpId,omitempty"`
	Group          string    `json:"group,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	Color          int       `json:"color"`
	Notes          string    `json:"notes,omitempty"`
	StartupCommand string    `json:"startupCommand,omitempty"`
	ForwardAgent   bool      `json:"forwardAgent,omitempty"`
	Created        time.Time `json:"created"`
	LastUsed       time.Time `json:"lastUsed,omitzero"`
}

func (h Host) Title() string {
	if h.Label != "" {
		return h.Label
	}

	return h.Address
}

func (h Host) Target() string {
	s := h.Address

	if h.Username != "" {
		s = h.Username + "@" + s
	}

	if h.Port != 0 && h.Port != 22 {
		s += ":" + itoa(h.Port)
	}

	return s
}

type Key struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Type        string    `json:"type"`
	Private     string    `json:"private"`
	Public      string    `json:"public"`
	Passphrase  string    `json:"passphrase,omitempty"`
	Certificate string    `json:"certificate,omitempty"`
	Created     time.Time `json:"created"`
}

type Forward struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	HostID   string `json:"hostId"`
	Kind     string `json:"kind"`
	BindAddr string `json:"bindAddr"`
	BindPort int    `json:"bindPort"`
	DestHost string `json:"destHost,omitempty"`
	DestPort int    `json:"destPort,omitempty"`
}

type Snippet struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Command  string `json:"command"`
	Shortcut string `json:"shortcut,omitempty"`
	Trigger  string `json:"trigger,omitempty"`
}

type KnownHost struct {
	Host        string    `json:"host"`
	KeyType     string    `json:"keyType"`
	Key         string    `json:"key"`
	Fingerprint string    `json:"fingerprint"`
	Added       time.Time `json:"added"`
}

type Data struct {
	Version    int         `json:"version"`
	Hosts      []Host      `json:"hosts"`
	Keys       []Key       `json:"keys"`
	Forwards   []Forward   `json:"forwards"`
	Snippets   []Snippet   `json:"snippets"`
	KnownHosts []KnownHost `json:"knownHosts"`
	Folders    []string    `json:"folders,omitempty"`
}

type Vault struct {
	mu     sync.RWMutex
	path   string
	data   Data
	rev    uint64
	env    envelope
	key    []byte
	locked bool
}

var ErrNotFound = errors.New("vault: not found")

func Open() (*Vault, error) {
	dir, err := config.Dir()

	if err != nil {
		return nil, err
	}

	return OpenAt(filepath.Join(dir, vaultFile))
}

func (v *Vault) Revision() uint64 {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.rev
}

func (v *Vault) Snapshot() Data {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return Data{
		Version:    v.data.Version,
		Hosts:      slices.Clone(v.data.Hosts),
		Keys:       slices.Clone(v.data.Keys),
		Forwards:   slices.Clone(v.data.Forwards),
		Snippets:   slices.Clone(v.data.Snippets),
		KnownHosts: slices.Clone(v.data.KnownHosts),
		Folders:    slices.Clone(v.data.Folders),
	}
}

func (v *Vault) update(fn func(d *Data)) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.locked {
		return ErrLocked
	}

	fn(&v.data)
	v.rev++
	return v.save()
}

func NewID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (v *Vault) Host(id string) (Host, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, h := range v.data.Hosts {
		if h.ID == id {
			return h, true
		}
	}

	return Host{}, false
}

func (v *Vault) SaveHost(h Host) (Host, error) {
	if h.ID == "" {
		h.ID = NewID()
		h.Created = time.Now()
	}

	if h.Port == 0 {
		h.Port = 22
	}

	err := v.update(func(d *Data) {
		d.Hosts = upsert(d.Hosts, h, func(x Host) string { return x.ID })
	})
	return h, err
}

func (v *Vault) DeleteHost(id string) error {
	return v.update(func(d *Data) {
		d.Hosts = slices.DeleteFunc(d.Hosts, func(h Host) bool { return h.ID == id })
		d.Forwards = slices.DeleteFunc(d.Forwards, func(f Forward) bool { return f.HostID == id })

		for i := range d.Hosts {
			if d.Hosts[i].JumpID == id {
				d.Hosts[i].JumpID = ""
			}
		}
	})
}

func (v *Vault) TouchHost(id string) error {
	return v.update(func(d *Data) {
		for i := range d.Hosts {
			if d.Hosts[i].ID == id {
				d.Hosts[i].LastUsed = time.Now()
			}
		}
	})
}

func (v *Vault) Key(id string) (Key, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, k := range v.data.Keys {
		if k.ID == id {
			return k, true
		}
	}

	return Key{}, false
}

func (v *Vault) SaveKey(k Key) (Key, error) {
	if k.ID == "" {
		k.ID = NewID()
		k.Created = time.Now()
	}

	err := v.update(func(d *Data) {
		d.Keys = upsert(d.Keys, k, func(x Key) string { return x.ID })
	})
	return k, err
}

func (v *Vault) DeleteKey(id string) error {
	return v.update(func(d *Data) {
		d.Keys = slices.DeleteFunc(d.Keys, func(k Key) bool { return k.ID == id })

		for i := range d.Hosts {
			if d.Hosts[i].KeyID == id {
				d.Hosts[i].KeyID = ""
				d.Hosts[i].Auth = AuthAgent
			}
		}
	})
}

func (v *Vault) SaveForward(f Forward) (Forward, error) {
	if f.ID == "" {
		f.ID = NewID()
	}

	err := v.update(func(d *Data) {
		d.Forwards = upsert(d.Forwards, f, func(x Forward) string { return x.ID })
	})
	return f, err
}

func (v *Vault) DeleteForward(id string) error {
	return v.update(func(d *Data) {
		d.Forwards = slices.DeleteFunc(d.Forwards, func(f Forward) bool { return f.ID == id })
	})
}

func (v *Vault) SaveSnippet(s Snippet) (Snippet, error) {
	if s.ID == "" {
		s.ID = NewID()
	}

	err := v.update(func(d *Data) {
		d.Snippets = upsert(d.Snippets, s, func(x Snippet) string { return x.ID })
	})
	return s, err
}

func (v *Vault) DeleteSnippet(id string) error {
	return v.update(func(d *Data) {
		d.Snippets = slices.DeleteFunc(d.Snippets, func(s Snippet) bool { return s.ID == id })
	})
}

func (v *Vault) KnownHost(host string) (KnownHost, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, k := range v.data.KnownHosts {
		if k.Host == host {
			return k, true
		}
	}

	return KnownHost{}, false
}

func (v *Vault) TrustHost(k KnownHost) error {
	k.Added = time.Now()
	return v.update(func(d *Data) {
		d.KnownHosts = upsert(d.KnownHosts, k, func(x KnownHost) string { return x.Host })
	})
}

func (v *Vault) ForgetHost(host string) error {
	return v.update(func(d *Data) {
		d.KnownHosts = slices.DeleteFunc(d.KnownHosts, func(k KnownHost) bool { return k.Host == host })
	})
}

func (d Data) Groups() []string {
	seen := map[string]struct{}{}
	var out []string

	for _, h := range d.Hosts {
		if _, ok := seen[h.Group]; ok || h.Group == "" {
			continue
		}

		seen[h.Group] = struct{}{}
		out = append(out, h.Group)
	}

	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func upsert[T any](list []T, item T, id func(T) string) []T {
	for i := range list {
		if id(list[i]) == id(item) {
			list[i] = item
			return list
		}
	}

	return append(list, item)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var b [20]byte
	i := len(b)
	neg := n < 0

	if neg {
		n = -n
	}

	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		b[i] = '-'
	}

	return string(b[i:])
}
