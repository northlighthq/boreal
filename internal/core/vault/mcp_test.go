package vault

import (
	"path/filepath"
	"testing"
)

func TestMCPPolicyAllows(t *testing.T) {
	prod := Host{ID: "1", Group: "Production"}
	web := Host{ID: "2", Group: "Production/Web"}
	near := Host{ID: "3", Group: "Production-old"}
	root := Host{ID: "4"}

	cases := []struct {
		name string
		p    MCPPolicy
		h    Host
		want bool
	}{
		{"off by default", MCPPolicy{}, prod, false},
		{"off even with all hosts", MCPPolicy{AllHosts: true}, prod, false},
		{"enabled without scope reaches nothing", MCPPolicy{Enabled: true}, prod, false},
		{"all hosts", MCPPolicy{Enabled: true, AllHosts: true}, root, true},
		{"folder", MCPPolicy{Enabled: true, Folders: []string{"Production"}}, prod, true},
		{"subfolder", MCPPolicy{Enabled: true, Folders: []string{"Production"}}, web, true},
		{"similar name is not inside", MCPPolicy{Enabled: true, Folders: []string{"Production"}}, near, false},
		{"root is not a folder prefix", MCPPolicy{Enabled: true, Folders: []string{"Production"}}, root, false},
		{"hosts without a folder", MCPPolicy{Enabled: true, Folders: []string{RootFolder}}, root, true},
		{"hosts without a folder only", MCPPolicy{Enabled: true, Folders: []string{RootFolder}}, prod, false},
		{"empty folder name is ignored", MCPPolicy{Enabled: true, Folders: []string{""}}, root, false},
	}

	for _, c := range cases {
		if got := c.p.Allows(c.h); got != c.want {
			t.Errorf("%s: Allows = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMCPPolicyStaysOutOfSnapshotsAndImports(t *testing.T) {
	v, err := OpenAt(filepath.Join(t.TempDir(), FileName))

	if err != nil {
		t.Fatal(err)
	}

	if v.MCP().Enabled {
		t.Fatal("a new vault must start with MCP off")
	}

	if err := v.SetMCP(MCPPolicy{Enabled: true, Read: true, Folders: []string{"B", "A", "A"}}); err != nil {
		t.Fatal(err)
	}

	if got := v.MCP(); !got.Enabled || len(got.Folders) != 2 || got.Folders[0] != "A" {
		t.Fatalf("policy = %+v", got)
	}

	if v.Snapshot().MCP.Enabled {
		t.Fatal("snapshots are what exports are built from, so they must leave the MCP policy out")
	}

	other, err := OpenAt(filepath.Join(t.TempDir(), FileName))

	if err != nil {
		t.Fatal(err)
	}

	if _, err := other.Merge(Data{MCP: MCPPolicy{Enabled: true, AllHosts: true, Access: true}}); err != nil {
		t.Fatal(err)
	}

	if other.MCP().Enabled {
		t.Fatal("an imported file must never turn MCP on")
	}
}

func TestMCPPolicySurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	v, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if err := v.SetMCP(MCPPolicy{Enabled: true, Read: true, AllHosts: true}); err != nil {
		t.Fatal(err)
	}

	again, err := OpenAt(path)

	if err != nil {
		t.Fatal(err)
	}

	if p := again.MCP(); !p.Enabled || !p.Read || !p.AllHosts || p.Access {
		t.Fatalf("policy after reopen = %+v", p)
	}
}
