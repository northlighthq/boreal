package access

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func newKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		t.Fatal(err)
	}

	k, err := ssh.NewPublicKey(pub)

	if err != nil {
		t.Fatal(err)
	}

	return k
}

func authorized(k ssh.PublicKey, rest string) string {
	return rest + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
}

func TestParse(t *testing.T) {
	a, b := newKey(t), newKey(t)
	content := "# managed by hand\n\n" +
		authorized(a, "") + " laptop\n" +
		authorized(b, `from="10.0.0.0/8",no-pty `) + " ci\n" +
		"this line is broken\n"

	got := Parse([]byte(content))

	if len(got) != 2 {
		t.Fatalf("entries = %+v", got)
	}

	if got[0].Fingerprint != ssh.FingerprintSHA256(a) || got[0].Comment != "laptop" || got[0].Line != 3 {
		t.Fatalf("first = %+v", got[0])
	}

	if len(got[1].Options) != 2 || got[1].Options[0] != `from="10.0.0.0/8"` || got[1].Options[1] != "no-pty" {
		t.Fatalf("options = %q", got[1].Options)
	}
}

func TestGrantAppendsOnceAndKeepsTheRest(t *testing.T) {
	a, b := newKey(t), newKey(t)
	content := []byte("# keep me\n" + authorized(a, "") + " laptop\n")

	next, changed, err := Grant(content, b, "ci runner", Restrictions{}, now)

	if err != nil || !changed {
		t.Fatalf("grant: changed=%v err=%v", changed, err)
	}

	text := string(next)

	if !strings.HasPrefix(text, "# keep me\n"+authorized(a, "")+" laptop\n") || !strings.HasSuffix(text, "ci runner (boreal)\n") {
		t.Fatalf("content = %q", text)
	}

	again, changed, err := Grant(next, b, "ci runner", Restrictions{}, now)

	if err != nil || changed || string(again) != text {
		t.Fatalf("granting twice must change nothing: changed=%v err=%v", changed, err)
	}
}

func TestGrantUpdatesRestrictionsInPlace(t *testing.T) {
	a, b := newKey(t), newKey(t)
	content := []byte(authorized(b, "") + " old\n" + authorized(a, "") + " laptop\n")

	next, changed, err := Grant(content, b, "ci", Restrictions{From: []string{"10.1.2.0/24"}, NoPTY: true}, now)

	if err != nil || !changed {
		t.Fatalf("grant: changed=%v err=%v", changed, err)
	}

	entries := Parse(next)

	if len(entries) != 2 || entries[0].Fingerprint != ssh.FingerprintSHA256(b) || len(entries[0].Options) != 2 || entries[0].Line != 1 {
		t.Fatalf("the line must be updated where it was: %+v", entries)
	}
}

func TestGrantRefusesWhenTheKeyIsOnSeveralLines(t *testing.T) {
	b := newKey(t)
	content := []byte(authorized(b, `from="10.0.0.0/8" `) + " ci\n" + authorized(b, "") + " copy\n")

	if _, _, err := Grant(content, b, "ci", Restrictions{}, now); !errors.Is(err, ErrSeveral) {
		t.Fatalf("err = %v, want ErrSeveral", err)
	}
}

func TestCertificateAuthorityLinesAreLeftAlone(t *testing.T) {
	ca := newKey(t)
	caLine := authorized(ca, `cert-authority,principals="ops" `) + " ops ca"
	content := []byte(caLine + "\n")

	next, changed, err := Grant(content, ca, "ca as a user key", Restrictions{}, now)

	if err != nil || !changed {
		t.Fatalf("grant: changed=%v err=%v", changed, err)
	}

	if !strings.HasPrefix(string(next), caLine+"\n") || len(Parse(next)) != 2 {
		t.Fatalf("the CA line must stay and a separate user line be added: %q", next)
	}

	after, removed := Revoke(next, ssh.FingerprintSHA256(ca))

	if removed != 1 || !strings.Contains(string(after), caLine) {
		t.Fatalf("revoke must remove the user line only: removed=%d %q", removed, after)
	}
}

func TestRevokeRemovesEveryCopy(t *testing.T) {
	a, b := newKey(t), newKey(t)
	content := []byte("# hi\n" + authorized(b, "") + " x\n" + authorized(a, "") + " keep\n" + authorized(b, "no-pty ") + " y\n")

	next, removed := Revoke(content, ssh.FingerprintSHA256(b))

	if removed != 2 {
		t.Fatalf("removed = %d", removed)
	}

	if want := "# hi\n" + authorized(a, "") + " keep\n"; string(next) != want {
		t.Fatalf("content = %q, want %q", next, want)
	}

	if _, removed := Revoke(next, ssh.FingerprintSHA256(b)); removed != 0 {
		t.Fatalf("second revoke removed %d", removed)
	}
}

func TestRestrictionsRejectInjection(t *testing.T) {
	cases := []struct {
		name string
		r    Restrictions
		want error
	}{
		{"quote in command", Restrictions{Command: `echo "hi"`}, ErrCommand},
		{"newline in command", Restrictions{Command: "true\nssh-ed25519 AAAA evil"}, ErrCommand},
		{"backslash in command", Restrictions{Command: `echo \" ok`}, ErrCommand},
		{"long command", Restrictions{Command: strings.Repeat("a", maxCommand+1)}, ErrCommand},
		{"comma in source", Restrictions{From: []string{"10.0.0.1,0.0.0.0/0"}}, ErrFrom},
		{"quote in source", Restrictions{From: []string{`10.0.0.1",command="sh`}}, ErrFrom},
		{"space in source", Restrictions{From: []string{"10.0.0.1 x"}}, ErrFrom},
		{"newline in source", Restrictions{From: []string{"10.0.0.1\nssh-rsa"}}, ErrFrom},
		{"expiry in the past", Restrictions{Expiry: "20200101"}, ErrExpiry},
		{"expiry not a date", Restrictions{Expiry: "20261340"}, ErrExpiry},
		{"expiry with letters", Restrictions{Expiry: "tomorrow"}, ErrExpiry},
	}

	for _, c := range cases {
		if err := c.r.Validate(now); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}

	ok := Restrictions{From: []string{"10.0.0.0/8", "*.example.com", "!10.0.0.9"}, Command: "/usr/bin/backup --run", Expiry: "202701011200", NoForwarding: true, NoPTY: true}

	if err := ok.Validate(now); err != nil {
		t.Fatalf("valid restrictions rejected: %v", err)
	}
}

func TestLineFormat(t *testing.T) {
	k := newKey(t)
	line := Line(k, "deploy\nkey", Restrictions{From: []string{"10.0.0.0/8"}, Expiry: "20270101", NoForwarding: true})

	if strings.ContainsAny(line, "\n\r") {
		t.Fatalf("line breaks leaked into %q", line)
	}

	want := `from="10.0.0.0/8",expiry-time="20270101",no-port-forwarding,no-agent-forwarding,no-X11-forwarding `

	if !strings.HasPrefix(line, want) || !strings.HasSuffix(line, " deploykey (boreal)") {
		t.Fatalf("line = %q", line)
	}

	parsed := Parse([]byte(line))

	if len(parsed) != 1 || parsed[0].Fingerprint != ssh.FingerprintSHA256(k) || len(parsed[0].Options) != 5 {
		t.Fatalf("the line Boreal writes must parse back: %+v", parsed)
	}
}

func TestGrantRejectsInvalidRestrictions(t *testing.T) {
	k := newKey(t)

	if _, _, err := Grant(nil, k, "x", Restrictions{Command: "a\"b"}, now); !errors.Is(err, ErrCommand) {
		t.Fatalf("err = %v", err)
	}
}
