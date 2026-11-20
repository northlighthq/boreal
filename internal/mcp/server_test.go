package mcp

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/ssh"

	"github.com/northlighthq/boreal/internal/core/access"
	"github.com/northlighthq/boreal/internal/core/vault"
)

const (
	secretPassword = "hunter2-very-secret"
	secretPrivate  = "-----BEGIN OPENSSH PRIVATE KEY-----SECRETDATA"
	secretProxy    = "nc proxy.internal 22 --token SECRETTOKEN"
)

type fakeFiles struct {
	content  []byte
	replaced int
}

func (f *fakeFiles) Read() ([]byte, error) { return bytes.Clone(f.content), nil }

func (f *fakeFiles) Replace(old, next []byte) error {
	if !bytes.Equal(old, f.content) {
		return access.ErrChanged
	}

	f.content = bytes.Clone(next)
	f.replaced++
	return nil
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

type fakeView struct {
	policy vault.MCPPolicy
	data   vault.Data
	files  map[string]*fakeFiles
	dials  []string
}

func (v *fakeView) Policy() vault.MCPPolicy { return v.policy }
func (v *fakeView) Data() vault.Data         { return v.data }

func (v *fakeView) Files(_ context.Context, h vault.Host) (access.Files, io.Closer, error) {
	v.dials = append(v.dials, h.ID)
	return v.files[h.ID], nopCloser{}, nil
}

type memLog struct{ records []Record }

func (m *memLog) Record(r Record) { m.records = append(m.records, r) }

func pubKey(t *testing.T) (ssh.PublicKey, string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		t.Fatal(err)
	}

	k, err := ssh.NewPublicKey(pub)

	if err != nil {
		t.Fatal(err)
	}

	return k, string(ssh.MarshalAuthorizedKey(k))
}

type fixture struct {
	view   *fakeView
	log    *memLog
	client *mcp.ClientSession
	own    ssh.PublicKey
	team   ssh.PublicKey
}

func setup(t *testing.T, p vault.MCPPolicy) *fixture {
	t.Helper()
	own, ownLine := pubKey(t)
	team, teamLine := pubKey(t)

	data := vault.Data{
		Hosts: []vault.Host{
			{ID: "web", Label: "web-01", Address: "10.0.0.1", Port: 22, Username: "deploy", Auth: vault.AuthKey, KeyID: "own", Group: "Production", Password: secretPassword, ProxyCommand: secretProxy},
			{ID: "db", Label: "db-01", Address: "10.0.0.2", Username: "root", Auth: vault.AuthPassword, Password: secretPassword, Group: "Private"},
		},
		Keys: []vault.Key{
			{ID: "own", Label: "Boreal key", Type: "ssh-ed25519", Public: ownLine, Private: secretPrivate, Passphrase: secretPassword},
			{ID: "team", Label: "Teammate", Type: "ssh-ed25519", Public: teamLine, Private: secretPrivate},
		},
		Snippets: []vault.Snippet{{ID: "s", Label: "Disk", Command: "df -h"}},
	}

	f := &fixture{
		view: &fakeView{policy: p, data: data, files: map[string]*fakeFiles{
			"web": {content: []byte(ownLine)},
			"db":  {content: []byte(ownLine)},
		}},
		log:  &memLog{},
		own:  own,
		team: team,
	}

	srv := New(func() (View, error) { return f.view, nil }, f.log)
	srv.now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()

	if _, err := srv.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { cs.Close() })
	f.client = cs
	return f
}

func (f *fixture) call(t *testing.T, tool string, args any) (string, bool) {
	t.Helper()
	res, err := f.client.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})

	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}

	var b strings.Builder

	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}

	return b.String(), res.IsError
}

var readOnly = vault.MCPPolicy{Enabled: true, Read: true, Folders: []string{"Production"}}
var withAccess = vault.MCPPolicy{Enabled: true, Read: true, Access: true, Folders: []string{"Production"}}

func TestEveryToolRefusesWhenOff(t *testing.T) {
	f := setup(t, vault.MCPPolicy{Read: true, Access: true, AllHosts: true})
	calls := map[string]any{
		toolListHosts:    map[string]any{},
		toolListKeys:     map[string]any{},
		toolPublicKey:    map[string]any{"key": "own"},
		toolListSnippets: map[string]any{},
		toolGetAccess:    map[string]any{"host": "web"},
		toolAuditAccess:  map[string]any{},
		toolGrantAccess:  map[string]any{"host": "web", "key": "team"},
		toolRevokeAccess: map[string]any{"host": "web", "key": "team"},
	}

	for tool, args := range calls {
		text, isErr := f.call(t, tool, args)

		if !isErr || !strings.Contains(text, "turned off") {
			t.Errorf("%s with MCP off: isError=%v text=%q", tool, isErr, text)
		}
	}

	if len(f.view.dials) != 0 {
		t.Fatalf("no host may be contacted while MCP is off, dialed %v", f.view.dials)
	}
}

func TestReadAndAccessNeedTheirOwnSwitch(t *testing.T) {
	f := setup(t, vault.MCPPolicy{Enabled: true, AllHosts: true})

	if text, isErr := f.call(t, toolListHosts, map[string]any{}); !isErr || !strings.Contains(text, "isn't allowed") {
		t.Fatalf("read without permission: %v %q", isErr, text)
	}

	f.view.policy = vault.MCPPolicy{Enabled: true, Read: true, AllHosts: true}

	if text, isErr := f.call(t, toolGrantAccess, map[string]any{"host": "web", "key": "team"}); !isErr || !strings.Contains(text, "changing which keys") {
		t.Fatalf("grant without permission: %v %q", isErr, text)
	}

	if f.view.files["web"].replaced != 0 {
		t.Fatal("authorized_keys was written without the access permission")
	}
}

func TestOnlySharedHostsAreVisible(t *testing.T) {
	f := setup(t, readOnly)
	text, isErr := f.call(t, toolListHosts, map[string]any{})

	if isErr || !strings.Contains(text, "web-01") || strings.Contains(text, "db-01") {
		t.Fatalf("list_hosts = %q", text)
	}

	if text, isErr := f.call(t, toolGetAccess, map[string]any{"host": "db-01"}); !isErr || !strings.Contains(text, "no host") {
		t.Fatalf("a host outside the scope must look missing: %v %q", isErr, text)
	}

	for _, id := range f.view.dials {
		if id == "db" {
			t.Fatal("a host outside the scope was contacted")
		}
	}
}

func TestSecretsNeverLeave(t *testing.T) {
	f := setup(t, vault.MCPPolicy{Enabled: true, Read: true, Access: true, AllHosts: true})
	var all strings.Builder

	for _, tool := range []string{toolListHosts, toolListKeys, toolListSnippets} {
		text, _ := f.call(t, tool, map[string]any{})
		all.WriteString(text)
	}

	text, _ := f.call(t, toolPublicKey, map[string]any{"key": "own"})
	all.WriteString(text)

	for _, secret := range []string{secretPassword, "SECRETDATA", "SECRETTOKEN"} {
		if strings.Contains(all.String(), secret) {
			t.Fatalf("a secret leaked through MCP: %q", secret)
		}
	}

	logged, _ := json.Marshal(f.log.records)

	for _, secret := range []string{secretPassword, "SECRETDATA", "SECRETTOKEN"} {
		if bytes.Contains(logged, []byte(secret)) {
			t.Fatalf("a secret leaked into the audit log: %q", secret)
		}
	}
}

func TestGrantAndRevoke(t *testing.T) {
	f := setup(t, withAccess)
	teamFP := ssh.FingerprintSHA256(f.team)

	text, isErr := f.call(t, toolGrantAccess, map[string]any{
		"host":         "web-01",
		"key":          "Teammate",
		"restrictions": map[string]any{"from": []string{"10.0.0.0/8"}, "noPty": true},
	})

	if isErr || !strings.Contains(text, `"changed":true`) {
		t.Fatalf("grant = %v %q", isErr, text)
	}

	file := f.view.files["web"]

	if !strings.Contains(string(file.content), `from="10.0.0.0/8",no-pty `) || !strings.Contains(string(file.content), "Teammate (boreal)") {
		t.Fatalf("authorized_keys = %q", file.content)
	}

	if _, isErr := f.call(t, toolGrantAccess, map[string]any{"host": "web-01", "key": "team", "restrictions": map[string]any{"from": []string{"10.0.0.0/8"}, "noPty": true}}); isErr || file.replaced != 1 {
		t.Fatalf("granting the same thing again must not rewrite the file, replaced=%d", file.replaced)
	}

	text, isErr = f.call(t, toolGetAccess, map[string]any{"host": "web"})

	if isErr || !strings.Contains(text, `"keyLabel":"Teammate"`) || !strings.Contains(text, `"keyLabel":"Boreal key"`) {
		t.Fatalf("get_access = %v %q", isErr, text)
	}

	text, isErr = f.call(t, toolRevokeAccess, map[string]any{"host": "web", "fingerprint": teamFP})

	if isErr || !strings.Contains(text, `"removed":1`) || strings.Contains(string(file.content), "Teammate") {
		t.Fatalf("revoke = %v %q, file %q", isErr, text, file.content)
	}
}

func TestRevokeRefusesTheKeyBorealSignsInWith(t *testing.T) {
	f := setup(t, withAccess)
	before := string(f.view.files["web"].content)

	text, isErr := f.call(t, toolRevokeAccess, map[string]any{"host": "web", "key": "own"})

	if !isErr || !strings.Contains(text, "lock Boreal out") {
		t.Fatalf("revoke own key = %v %q", isErr, text)
	}

	if string(f.view.files["web"].content) != before || len(f.view.dials) != 0 {
		t.Fatal("the host was touched while refusing")
	}
}

func TestGrantRejectsInjectionBeforeConnecting(t *testing.T) {
	f := setup(t, withAccess)
	attacks := []map[string]any{
		{"command": "true\nssh-ed25519 AAAA attacker"},
		{"command": `x",command="sh`},
		{"from": []string{`*",command="sh`}},
		{"from": []string{"0.0.0.0/0,10.0.0.1"}},
		{"expiry": "20200101"},
	}

	for _, r := range attacks {
		text, isErr := f.call(t, toolGrantAccess, map[string]any{"host": "web", "key": "team", "restrictions": r})

		if !isErr {
			t.Fatalf("restrictions %v were accepted: %q", r, text)
		}
	}

	if len(f.view.dials) != 0 || f.view.files["web"].replaced != 0 {
		t.Fatal("an invalid grant reached the server")
	}

	_, line := pubKey(t)

	if text, isErr := f.call(t, toolGrantAccess, map[string]any{"host": "web", "publicKey": line}); !isErr || !strings.Contains(text, "label") {
		t.Fatalf("a bare public key needs a label: %v %q", isErr, text)
	}

	if text, isErr := f.call(t, toolGrantAccess, map[string]any{"host": "web", "publicKey": "not a key", "label": "x"}); !isErr {
		t.Fatalf("garbage public key accepted: %q", text)
	}
}

func TestEveryCallIsLogged(t *testing.T) {
	f := setup(t, withAccess)
	f.call(t, toolListHosts, map[string]any{})
	f.call(t, toolRevokeAccess, map[string]any{"host": "web", "key": "own"})

	if len(f.log.records) != 2 {
		t.Fatalf("records = %+v", f.log.records)
	}

	last := f.log.records[1]

	if last.Tool != toolRevokeAccess || last.Outcome != OutcomeFail || last.Host != "web" || last.Key == "" {
		t.Fatalf("last record = %+v", last)
	}
}
