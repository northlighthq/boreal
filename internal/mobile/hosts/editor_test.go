package hosts

import (
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestAuthFor(t *testing.T) {
	cases := []struct {
		name     string
		keep     bool
		choice   int
		wantAuth string
		wantKey  string
		wantPass string
	}{
		{name: "keeps the agent", keep: true, choice: authPassword, wantAuth: vault.AuthAgent},
		{name: "key", choice: authKey, wantAuth: vault.AuthKey, wantKey: "k1"},
		{name: "password", choice: authPassword, wantAuth: vault.AuthPassword, wantPass: "secret"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			auth, key, pass := authFor(c.keep, c.choice, "k1", "secret")

			if auth != c.wantAuth || key != c.wantKey || pass != c.wantPass {
				t.Fatalf("authFor = %q, %q, %q; want %q, %q, %q", auth, key, pass, c.wantAuth, c.wantKey, c.wantPass)
			}
		})
	}
}

func TestKeepsAgent(t *testing.T) {
	e := NewEditor(nil, vault.Host{ID: "1", Auth: vault.AuthAgent, Agent: "~/.1password/agent.sock"})

	if !e.keepsAgent() {
		t.Fatal("an untouched agent host should keep the agent")
	}

	e.password.SetText("typed")

	if e.keepsAgent() {
		t.Fatal("typing a password should switch the host to it")
	}

	e.password.SetText("")
	e.authSet = true

	if e.keepsAgent() {
		t.Fatal("picking a method should switch the host to it")
	}

	if NewEditor(nil, vault.Host{ID: "2", Auth: vault.AuthPassword}).keepsAgent() {
		t.Fatal("only agent hosts keep the agent")
	}
}
