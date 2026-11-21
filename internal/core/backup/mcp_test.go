package backup

import (
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestExportsNeverCarryTheMCPPolicy(t *testing.T) {
	d := vault.Data{
		Hosts: []vault.Host{{ID: "1", Label: "web"}},
		MCP:   vault.MCPPolicy{Enabled: true, Read: true, Access: true, AllHosts: true},
	}
	all := Selection{Hosts: true, Keys: true, Secrets: true, Forwards: true, Snippets: true, KnownHosts: true}

	if Filter(d, all).MCP.Enabled {
		t.Fatal("the export filter kept the MCP policy")
	}

	raw, err := Export(d, all, testPassword)

	if err != nil {
		t.Fatal(err)
	}

	back, err := Import(raw, testPassword)

	if err != nil {
		t.Fatal(err)
	}

	if back.MCP.Enabled || back.MCP.Access {
		t.Fatalf("an exported file carried the MCP policy: %+v", back.MCP)
	}
}
