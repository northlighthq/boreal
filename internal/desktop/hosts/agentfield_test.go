package hosts

import (
	"errors"
	"testing"

	"github.com/northlighthq/boreal/internal/core/sshx"
)

func TestAgentStatus(t *testing.T) {
	op := sshx.AgentInfo{Path: "/home/me/.1password/agent.sock", Keys: make([]sshx.AgentKey, 3)}
	cases := []struct {
		info sshx.AgentInfo
		err  error
		want string
	}{
		{op, nil, "Found 1Password with 3 keys."},
		{sshx.AgentInfo{Path: op.Path, Keys: op.Keys[:1]}, nil, "Found 1Password with 1 key."},
		{sshx.AgentInfo{Path: op.Path}, errors.New("locked"), "Found 1Password, but it didn't list its keys. It may be locked."},
		{sshx.AgentInfo{}, sshx.ErrAgentOff, "The agent is off for this host."},
		{sshx.AgentInfo{}, sshx.ErrRemoteAgent, "Only an agent on this computer can be used."},
		{sshx.AgentInfo{}, sshx.ErrNoAgent, "No agent found."},
	}

	for _, c := range cases {
		if got := agentStatus(c.info, c.err); got != c.want {
			t.Errorf("agentStatus(%+v, %v) = %q, want %q", c.info, c.err, got, c.want)
		}
	}
}

func TestAgentKeyLabel(t *testing.T) {
	k := sshx.AgentKey{Type: "ssh-ed25519", Comment: "GitHub", Fingerprint: "SHA256:abcdefghijklmnopqrstuvwxyz"}

	if got := agentKeyLabel(k); got != "GitHub  ·  ED25519  ·  abcdefghijklmnop…" {
		t.Fatalf("label = %q", got)
	}

	k.Comment = ""

	if got := agentKeyLabel(k); got != "Unnamed key  ·  ED25519  ·  abcdefghijklmnop…" {
		t.Fatalf("label = %q", got)
	}
}

func TestAgentFieldKeepsMissingKey(t *testing.T) {
	var f agentField
	f.path.SetText("none")
	f.load("SHA256:gone")

	if f.chosen() != "SHA256:gone" {
		t.Fatalf("a saved key should stay selected while the agent is away, got %q", f.chosen())
	}

	if len(f.opts) != 2 || f.opts[0].Label != anyAgentKey {
		t.Fatalf("opts = %+v", f.opts)
	}

	f.key.Value = 0

	if f.chosen() != "" {
		t.Fatalf("picking any key should clear the choice, got %q", f.chosen())
	}
}
