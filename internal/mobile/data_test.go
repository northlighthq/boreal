package mobile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/platform/keyring"
)

func TestMain(m *testing.M) {
	keyring.UseMemory()
	os.Exit(m.Run())
}

func TestDataFollowsTheVaultAndIsForgotten(t *testing.T) {
	v, err := vault.OpenAt(filepath.Join(t.TempDir(), vault.FileName))

	if err != nil {
		t.Fatal(err)
	}

	a := &App{vault: v}

	if len(a.Data().Hosts) != 0 {
		t.Fatal("a new vault has no hosts")
	}

	first := a.Data()

	if _, err := v.SaveHost(vault.Host{Label: "web", Address: "10.0.0.1", Password: "secret"}); err != nil {
		t.Fatal(err)
	}

	if got := a.Data().Hosts; len(got) != 1 || got[0].Label != "web" {
		t.Fatalf("a saved host must show up on the next read, got %+v", got)
	}

	if len(first.Hosts) != 0 {
		t.Fatal("an earlier read changed under the caller")
	}

	a.forgetData()

	if a.data.Hosts != nil || a.dataOK {
		t.Fatal("forgetting must drop the cached hosts and their passwords")
	}
}
