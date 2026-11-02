package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func newKey(t *testing.T) (ssh.PublicKey, ssh.Signer) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		t.Fatal(err)
	}

	signer, err := ssh.NewSignerFromKey(priv)

	if err != nil {
		t.Fatal(err)
	}

	sshPub, err := ssh.NewPublicKey(pub)

	if err != nil {
		t.Fatal(err)
	}

	return sshPub, signer
}

func withHome(t *testing.T, lines ...string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ProgramData", t.TempDir())

	if len(lines) == 0 {
		return
	}

	path := filepath.Join(home, ".ssh", "known_hosts")

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

var remote = &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}

func TestSystemHashedEntry(t *testing.T) {
	key, _ := newKey(t)
	other, _ := newKey(t)
	withHome(t, knownhosts.Line([]string{knownhosts.HashHostname("web.example.com")}, key))

	if v, _ := checkSystem("web.example.com:22", remote, key); v != systemTrusted {
		t.Fatalf("hashed entry: got %v", v)
	}

	v, want := checkSystem("web.example.com:22", remote, other)

	if v != systemChanged || want == nil || string(want.Marshal()) != string(key.Marshal()) {
		t.Fatalf("changed key: got %v", v)
	}

	if v, _ := checkSystem("other.example.com:22", remote, key); v != systemUnknown {
		t.Fatalf("unknown host: got %v", v)
	}
}

func TestSystemRevoked(t *testing.T) {
	key, _ := newKey(t)
	withHome(t, "@revoked * "+strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))))

	if v, _ := checkSystem("web.example.com:22", remote, key); v != systemRevoked {
		t.Fatalf("got %v", v)
	}
}

func TestSystemCertAuthority(t *testing.T) {
	caPub, caSigner := newKey(t)
	hostPub, _ := newKey(t)
	withHome(t, "@cert-authority *.example.com "+strings.TrimSpace(string(ssh.MarshalAuthorizedKey(caPub))))

	cert := &ssh.Certificate{
		Key:             hostPub,
		CertType:        ssh.HostCert,
		ValidPrincipals: []string{"web.example.com"},
		ValidBefore:     ssh.CertTimeInfinity,
	}

	if err := cert.SignCert(rand.Reader, caSigner); err != nil {
		t.Fatal(err)
	}

	if v, _ := checkSystem("web.example.com:22", remote, cert); v != systemTrusted {
		t.Fatalf("got %v", v)
	}
}

func TestAddToKnownHosts(t *testing.T) {
	key, _ := newKey(t)
	withHome(t, "# existing line without newline")
	path := UserKnownHostsFile()
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.TrimSpace(string(raw))), 0o600)

	if err := addToKnownHosts("db.example.com:2222", key); err != nil {
		t.Fatal(err)
	}

	if v, _ := checkSystem("db.example.com:2222", remote, key); v != systemTrusted {
		t.Fatalf("got %v", v)
	}

	raw, _ = os.ReadFile(path)

	if !strings.Contains(string(raw), "\n[db.example.com]:2222 ssh-ed25519 ") {
		t.Fatalf("unexpected file:\n%s", raw)
	}
}
