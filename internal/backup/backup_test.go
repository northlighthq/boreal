package backup

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/secure"
	"github.com/northlighthq/boreal/internal/vault"
)

const testPassword = "a long enough password"

func TestRoundTrip(t *testing.T) {
	d := vault.Data{Hosts: []vault.Host{{ID: "h1", Address: "example.com"}}}
	raw, err := Export(d, Selection{Hosts: true}, testPassword)

	if err != nil {
		t.Fatal(err)
	}

	var f file

	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}

	if f.Format != fileFormat || f.Version != fileVersion {
		t.Fatalf("wrote %s v%d", f.Format, f.Version)
	}

	got, err := Import(raw, testPassword)

	if err != nil {
		t.Fatal(err)
	}

	if len(got.Hosts) != 1 || got.Hosts[0].Address != "example.com" {
		t.Fatalf("unexpected hosts: %+v", got.Hosts)
	}
}

func TestLegacyImport(t *testing.T) {
	plain, _ := json.Marshal(vault.Data{Snippets: []vault.Snippet{{ID: "s1", Command: "uptime"}}})
	kdf := secure.NewKDF(1, 8*1024)
	key, err := kdf.Key(testPassword)

	if err != nil {
		t.Fatal(err)
	}

	sealed, err := secure.Seal(key, plain, legacyAAD)

	if err != nil {
		t.Fatal(err)
	}

	raw, _ := json.Marshal(file{Format: legacyFormat, Version: 1, Created: time.Now(), KDF: kdf, Data: sealed})
	got, err := Import(raw, testPassword)

	if err != nil {
		t.Fatal(err)
	}

	if len(got.Snippets) != 1 || got.Snippets[0].Command != "uptime" {
		t.Fatalf("unexpected snippets: %+v", got.Snippets)
	}
}
