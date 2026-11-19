package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestLocalAgentPath(t *testing.T) {
	cases := map[string]bool{
		"":                                true,
		"~/.1password/agent.sock":         true,
		"/run/user/1000/ssh-agent.socket": true,
		`C:\Users\me\agent.sock`:          true,
		`\\.\pipe\openssh-ssh-agent`:      true,
		`\\.\PIPE\openssh-ssh-agent`:      true,
		`\\.\pipe\`:                       false,
		`\\server\pipe\openssh-ssh-agent`: false,
		`\\?\UNC\server\share\agent.sock`: false,
		`\??\UNC\server\share\agent.sock`: false,
		"//server/pipe/openssh-ssh-agent": false,
		`  \\server\share\agent.sock`:     false,
	}

	for p, want := range cases {
		if got := LocalAgentPath(p); got != want {
			t.Errorf("LocalAgentPath(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestAgentCandidates(t *testing.T) {
	if _, err := agentCandidates("none"); !errors.Is(err, ErrAgentOff) {
		t.Fatalf("none: err = %v", err)
	}

	if _, err := agentCandidates(`\\server\pipe\openssh-ssh-agent`); !errors.Is(err, ErrRemoteAgent) {
		t.Fatalf("remote pipe: err = %v", err)
	}

	t.Setenv(agentEnv, "")

	if _, err := agentCandidates(agentEnv); !errors.Is(err, ErrNoAgent) {
		t.Fatalf("empty SSH_AUTH_SOCK: err = %v", err)
	}

	t.Setenv(agentEnv, "/tmp/agent.sock")
	got, err := agentCandidates(agentEnv)

	if err != nil || len(got) != 1 || got[0] != "/tmp/agent.sock" {
		t.Fatalf("SSH_AUTH_SOCK = %v, %v", got, err)
	}
}

func newAgentKey(t *testing.T, kr agent.Agent, comment string) ssh.PublicKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		t.Fatal(err)
	}

	if err := kr.Add(agent.AddedKey{PrivateKey: priv, Comment: comment}); err != nil {
		t.Fatal(err)
	}

	key, err := ssh.NewPublicKey(pub)

	if err != nil {
		t.Fatal(err)
	}

	return key
}

func TestAgentSigners(t *testing.T) {
	kr := agent.NewKeyring()
	var keys []ssh.PublicKey

	for _, c := range []string{"a", "b", "c", "d", "e", "f"} {
		keys = append(keys, newAgentKey(t, kr, c))
	}

	var said []string
	progress := func(m string) { said = append(said, m) }

	all, err := agentSigners(kr, "", progress)()

	if err != nil || len(all) != 6 {
		t.Fatalf("all signers = %d, %v", len(all), err)
	}

	if len(said) != 1 {
		t.Fatalf("six keys should warn about the server's try limit, got %v", said)
	}

	want := ssh.FingerprintSHA256(keys[3])
	one, err := agentSigners(kr, want, progress)()

	if err != nil || len(one) != 1 || ssh.FingerprintSHA256(one[0].PublicKey()) != want {
		t.Fatalf("chosen key not picked: %d, %v", len(one), err)
	}

	said = nil
	none, err := agentSigners(kr, "SHA256:missing", progress)()

	if err != nil || len(none) != 0 || len(said) != 1 {
		t.Fatalf("missing key: %d signers, %v, said %v", len(none), err, said)
	}
}

func TestInspectAgentOverSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)

	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}

	defer ln.Close()
	kr := agent.NewKeyring()
	key := newAgentKey(t, kr, "work laptop")

	go func() {
		for {
			c, err := ln.Accept()

			if err != nil {
				return
			}

			go func() {
				defer c.Close()
				agent.ServeAgent(kr, c)
			}()
		}
	}()

	info, err := InspectAgent(sock)

	if err != nil {
		t.Fatal(err)
	}

	if info.Path != sock || len(info.Keys) != 1 {
		t.Fatalf("info = %+v", info)
	}

	k := info.Keys[0]

	if k.Comment != "work laptop" || k.Fingerprint != ssh.FingerprintSHA256(key) || k.Type != ssh.KeyAlgoED25519 {
		t.Fatalf("key = %+v", k)
	}

	if p, ok := AgentPath(sock); !ok || p != sock {
		t.Fatalf("AgentPath = %q, %v", p, ok)
	}
}
