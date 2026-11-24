package backup

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func FuzzParse(f *testing.F) {
	raw, err := Export(vault.Data{Hosts: []vault.Host{{ID: "h1", Address: "example.com"}}}, Selection{Hosts: true}, testPassword)

	if err != nil {
		f.Fatal(err)
	}

	f.Add(raw)
	f.Add([]byte(`{"format":"platty-export","version":1}`))
	f.Add([]byte(`{"format":"boreal-export","version":99}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		parse(raw)
	})
}

func FuzzDecodeAndMerge(f *testing.F) {
	seed, _ := json.Marshal(vault.Data{
		Hosts:      []vault.Host{{ID: "h1", Address: "example.com", Port: 22, Group: "Production/Web", JumpID: "h1", KeyID: "k1"}},
		Keys:       []vault.Key{{ID: "k1", Label: "key"}},
		Forwards:   []vault.Forward{{ID: "f1", HostID: "h1", Kind: vault.ForwardLocal, BindPort: 8080}},
		Snippets:   []vault.Snippet{{ID: "s1", Command: "uptime"}},
		KnownHosts: []vault.KnownHost{{Host: "example.com:22", KeyType: "ssh-ed25519"}},
		Folders:    []string{"Production", "Production/Web", ""},
	})

	f.Add(seed)
	f.Add([]byte(`{"hosts":[{"id":""},{"id":""}],"folders":["/","//","a//b"]}`))
	f.Add([]byte(`{"knownHosts":[{"host":""},{"host":""}]}`))

	f.Fuzz(func(t *testing.T, plain []byte) {
		d, err := decode(plain)

		if err != nil {
			return
		}

		v, err := vault.OpenAt(filepath.Join(t.TempDir(), "vault.dat"))

		if err != nil {
			t.Fatal(err)
		}

		if _, err := v.Merge(d); err != nil {
			t.Fatal(err)
		}

		snap := v.Snapshot()
		snap.Groups()
		SSHConfig(snap.Hosts)
		Filter(snap, Selection{Hosts: true, Keys: true, Forwards: true, Snippets: true, KnownHosts: true})
	})
}
