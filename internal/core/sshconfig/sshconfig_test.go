package sshconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func entry(t *testing.T, res Result, alias string) Entry {
	t.Helper()

	for _, e := range res.Entries {
		if e.Alias == alias {
			return e
		}
	}

	t.Fatalf("no entry %q in %+v", alias, res.Entries)
	return Entry{}
}

func TestWildcardsAndFirstValueWins(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "config", `
Host web-*
    User deploy
    IdentitiesOnly yes

Host web-01 web-02
    HostName %h.example.com
    User root

Host db !db-legacy
    Port 2222

Host db-legacy
    HostName 10.0.0.9

Host *
    User fallback
    Port 2200
    ForwardAgent yes
`)

	res, err := Parse(path)

	if err != nil {
		t.Fatal(err)
	}

	web := entry(t, res, "web-02")

	if web.HostName != "web-02.example.com" || web.User != "deploy" || !web.IdentitiesOnly || web.Port != 2200 || !web.ForwardAgent {
		t.Fatalf("web-02: %+v", web)
	}

	db := entry(t, res, "db")

	if db.Port != 2222 || db.User != "fallback" {
		t.Fatalf("db: %+v", db)
	}

	legacy := entry(t, res, "db-legacy")

	if legacy.Port != 2200 || legacy.HostName != "10.0.0.9" {
		t.Fatalf("db-legacy: %+v", legacy)
	}

	if slices.ContainsFunc(res.Entries, func(e Entry) bool { return e.Alias == "*" || e.Alias == "web-*" }) {
		t.Fatal("wildcard patterns must not become hosts")
	}
}

func TestIncludeAndMatch(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "conf.d/a.conf", "Host inc-a\n    HostName a.internal\n")
	write(t, dir, "conf.d/b.conf", "User from-include\n\nHost inc-b\n    HostName b.internal\n")
	path := write(t, dir, "config", `
Host inc-a
    Include `+filepath.Join(dir, "conf.d", "b.conf")+`
    Port 2022

Host *
Include `+filepath.Join(dir, "conf.d", "*.conf")+`

Match host *.internal user root
    Port 9999

Match originalhost inc-b
    Port 2201

Match exec "test -f /tmp/x"
    Port 1
`)

	res, err := Parse(path)

	if err != nil {
		t.Fatal(err)
	}

	a := entry(t, res, "inc-a")

	if a.HostName != "a.internal" || a.User != "from-include" || a.Port != 2022 {
		t.Fatalf("inc-a: %+v", a)
	}

	b := entry(t, res, "inc-b")

	if b.HostName != "b.internal" || b.User != "from-include" || b.Port != 2201 {
		t.Fatalf("inc-b: %+v", b)
	}

	if !slices.ContainsFunc(res.Skipped, func(s Skipped) bool { return s.Name == "Match exec" }) {
		t.Fatalf("Match exec not reported: %+v", res.Skipped)
	}
}

func TestProxyAndReport(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "config", `
Host inner
    ProxyJump bastion,admin@gw.example.com:2222
    ProxyCommand nc %h %p
    IdentityFile ~/.ssh/id_%r
    User ops
    CertificateFile %d/.ssh/id_ops-cert.pub
    LocalForward 8080 localhost:80
    ServerAliveInterval 30

Host tunnel
    ProxyCommand "cloudflared" access ssh --hostname %h
    ControlMaster auto
    LocalForward 9090 localhost:90
`)

	res, err := Parse(path)

	if err != nil {
		t.Fatal(err)
	}

	inner := entry(t, res, "inner")
	home, _ := os.UserHomeDir()

	if !slices.Equal(inner.Jump, []string{"bastion", "admin@gw.example.com:2222"}) || inner.ProxyCommand != "" {
		t.Fatalf("inner proxy: %+v", inner)
	}

	if inner.Identity != filepath.Join(home, ".ssh", "id_ops") {
		t.Fatalf("identity: %q", inner.Identity)
	}

	if inner.Certificate != home+"/.ssh/id_ops-cert.pub" {
		t.Fatalf("certificate: %q", inner.Certificate)
	}

	tunnel := entry(t, res, "tunnel")

	if tunnel.ProxyCommand != "cloudflared access ssh --hostname %h" {
		t.Fatalf("proxy command: %q", tunnel.ProxyCommand)
	}

	want := []Skipped{{"ControlMaster", 1}, {"LocalForward", 2}, {"ServerAliveInterval", 1}}

	if !slices.Equal(res.Skipped, want) {
		t.Fatalf("skipped: %+v", res.Skipped)
	}
}

func TestFormatRoundTrip(t *testing.T) {
	in := []Entry{
		{Alias: "a", HostName: "a.example.com", User: "me", Port: 2222, Jump: []string{"b", "c"}, IdentitiesOnly: true},
		{Alias: "b", HostName: "b.example.com", Port: 22, ProxyCommand: "nc %h %p", ForwardAgent: true},
	}

	path := write(t, t.TempDir(), "config", Format(in))
	res, err := Parse(path)

	if err != nil {
		t.Fatal(err)
	}

	a, b := entry(t, res, "a"), entry(t, res, "b")

	if a.Port != 2222 || !slices.Equal(a.Jump, []string{"b", "c"}) || !a.IdentitiesOnly || a.User != "me" {
		t.Fatalf("a: %+v", a)
	}

	if b.ProxyCommand != "nc %h %p" || !b.ForwardAgent {
		t.Fatalf("b: %+v", b)
	}
}

func TestGlob(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "", true},
		{"web-*", "web-01", true},
		{"web-??", "web-01", true},
		{"web-?", "web-01", false},
		{"*.example.com", "a.b.example.com", true},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "aXbY", false},
		{"*a*a*a*a*a*a*a*a*b", strings.Repeat("a", 200), false},
	}

	for _, c := range cases {
		if got := glob(c.pattern, c.s); got != c.want {
			t.Errorf("glob(%q, %q) = %v", c.pattern, c.s, got)
		}
	}
}

func TestIncludeLoopStops(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	write(t, dir, "config", "Include "+path+"\nHost x\n")

	res, err := Parse(path)

	if err != nil {
		t.Fatal(err)
	}

	if len(res.Entries) != 1 {
		t.Fatalf("entries: %+v", res.Entries)
	}
}

func FuzzParse(f *testing.F) {
	f.Add("Host a b *.x !c\n  HostName %h.%%\n  Port=22\nMatch !host a,b user \"x y\"\nInclude nothing\n")
	f.Add("Host=a\n\tProxyJump a@b:c,,d\n\tProxyCommand none\n")

	f.Fuzz(func(t *testing.T, body string) {
		if strings.Contains(strings.ToLower(body), keyInclude) {
			t.Skip("Include reads the real file system")
		}

		path := filepath.Join(t.TempDir(), "config")

		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		res, err := Parse(path)

		if err != nil {
			return
		}

		Format(res.Entries)
	})
}
