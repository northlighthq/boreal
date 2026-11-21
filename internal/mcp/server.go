package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/ssh"

	"github.com/northlighthq/boreal/internal/core/access"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/core/version"
)

const (
	serverName   = "boreal"
	maxAudit     = 25
	defaultPort  = 22
	settingsPath = "Boreal › Settings › MCP"

	toolListHosts    = "list_hosts"
	toolListKeys     = "list_keys"
	toolPublicKey    = "get_public_key"
	toolListSnippets = "list_snippets"
	toolGetAccess    = "get_access"
	toolAuditAccess  = "audit_access"
	toolGrantAccess  = "grant_access"
	toolRevokeAccess = "revoke_access"
)

var (
	errOff       = errors.New("MCP is turned off. It can only be turned on in " + settingsPath)
	errNoRead    = errors.New("reading hosts and keys isn't allowed. It can be allowed in " + settingsPath)
	errNoAccess  = errors.New("changing which keys can sign in isn't allowed. It can be allowed in " + settingsPath)
	errNoScope   = errors.New("no hosts are shared over MCP. Choose them in " + settingsPath)
	errOneKey    = errors.New("give either key (a key saved in Boreal) or publicKey, not both")
	errNoKey     = errors.New("give key (a key saved in Boreal) or publicKey")
	errLockout   = errors.New("this is the key Boreal signs in to this host with. Revoking it would lock Boreal out, so change the host's key in Boreal first")
	errOwnKey    = errors.New("this is the key Boreal signs in to this host with. Changing its restrictions could lock Boreal out, so change it on the server instead")
	errNoLabel   = errors.New("give a label for this public key, like the person or machine it belongs to")
	errRevokeArg = errors.New("give key (a key saved in Boreal) or fingerprint")
)

type Server struct {
	backend Backend
	log     Recorder
	now     func() time.Time
	tun     tunnels
}

func New(b Backend, log Recorder) *Server {
	return &Server{backend: b, log: log, now: time.Now, tun: tunnels{minute: time.Minute, recheck: tunnelRecheck, quit: make(chan struct{})}}
}

type tier int

const (
	tierRead tier = iota
	tierAccess
	tierExecute
	tierFiles
	tierWriteFiles
	tierTunnels
)

func (s *Server) load(t tier) (View, error) {
	v, err := s.backend()

	if err != nil {
		return nil, err
	}

	p := v.Policy()

	switch {
	case !p.Enabled:
		return nil, errOff
	case !p.Read:
		return nil, errNoRead
	case t == tierAccess && !p.Access:
		return nil, errNoAccess
	case t == tierExecute && !p.Execute:
		return nil, errNoExecute
	case (t == tierFiles || t == tierWriteFiles) && !p.Files:
		return nil, errNoFiles
	case t == tierWriteFiles && !p.WriteFiles:
		return nil, errNoWrite
	case t == tierTunnels && !p.Tunnels:
		return nil, errNoTunnels
	case !p.Scoped():
		return nil, errNoScope
	}

	return v, nil
}

func (s *Server) record(tool, host, key string, err error) {
	r := Record{Time: s.now(), Tool: tool, Host: host, Key: key, Outcome: OutcomeOK}

	if err != nil {
		r.Outcome, r.Error = OutcomeFail, err.Error()
	}

	s.log.Record(r)
}

func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: version.Version}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	closed := false
	destructive := true

	mcp.AddTool(srv, &mcp.Tool{Name: toolListHosts, Description: "List the SSH hosts saved in Boreal that are shared over MCP. Passwords, proxy commands and startup commands are never included.", Annotations: readOnly}, s.listHosts)
	mcp.AddTool(srv, &mcp.Tool{Name: toolListKeys, Description: "List the SSH keys saved in Boreal: label, type and fingerprint. Private keys and passphrases are never included.", Annotations: readOnly}, s.listKeys)
	mcp.AddTool(srv, &mcp.Tool{Name: toolPublicKey, Description: "Get the public key of a key saved in Boreal, in authorized_keys format.", Annotations: readOnly}, s.publicKey)
	mcp.AddTool(srv, &mcp.Tool{Name: toolListSnippets, Description: "List the command snippets saved in Boreal.", Annotations: readOnly}, s.listSnippets)
	mcp.AddTool(srv, &mcp.Tool{Name: toolGetAccess, Description: "Connect to a host and list the keys in its ~/.ssh/authorized_keys, with their restrictions and the Boreal key each one matches.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, s.getAccess)
	mcp.AddTool(srv, &mcp.Tool{Name: toolAuditAccess, Description: fmt.Sprintf("List the authorized keys of every shared host, or of one folder, in one call. Checks at most %d hosts.", maxAudit), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, s.auditAccess)
	mcp.AddTool(srv, &mcp.Tool{Name: toolGrantAccess, Description: "Let a key sign in to a host by adding it to the host's ~/.ssh/authorized_keys, optionally restricted by source address, a single command, an expiry date or no forwarding. If the key is already there its restrictions are replaced. Boreal backs up the file first.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: &destructive, OpenWorldHint: &closed}}, s.grantAccess)
	mcp.AddTool(srv, &mcp.Tool{Name: toolRevokeAccess, Description: "Stop a key from signing in to a host by removing it from the host's ~/.ssh/authorized_keys. Refuses to remove the key Boreal itself signs in with. Boreal backs up the file first.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: &destructive, OpenWorldHint: &closed}}, s.revokeAccess)

	s.addExec(srv)
	s.addFiles(srv)
	s.addTunnels(srv)
	return srv
}

type HostOut struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Address string   `json:"address"`
	Port    int      `json:"port"`
	User    string   `json:"user"`
	Folder  string   `json:"folder,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	OS      string   `json:"os,omitempty"`
	Auth    string   `json:"auth"`
	Jump    string   `json:"jump,omitempty"`
}

type ListHostsIn struct {
	Folder string `json:"folder,omitempty" jsonschema:"only hosts in this folder and its subfolders"`
	Query  string `json:"query,omitempty" jsonschema:"only hosts whose label, address or tags contain this text"`
}

type ListHostsOut struct {
	Hosts []HostOut `json:"hosts"`
}

func shared(v View) []vault.Host {
	p := v.Policy()
	var out []vault.Host

	for _, h := range v.Data().Hosts {
		if p.Allows(h) {
			out = append(out, h)
		}
	}

	return out
}

func hostOut(h vault.Host, d vault.Data, p vault.MCPPolicy) HostOut {
	o := HostOut{ID: h.ID, Label: h.Title(), Address: h.Address, Port: h.Port, User: h.Username, Folder: h.Group, Tags: h.Tags, OS: h.OS, Auth: h.Auth}

	if o.Port == 0 {
		o.Port = 22
	}

	if i := slices.IndexFunc(d.Hosts, func(x vault.Host) bool { return x.ID == h.JumpID }); i >= 0 && p.Allows(d.Hosts[i]) {
		o.Jump = d.Hosts[i].Title()
	}

	return o
}

func inFolder(h vault.Host, folder string) bool {
	return folder == "" || h.Group == folder || strings.HasPrefix(h.Group, folder+vault.FolderSep)
}

func (s *Server) listHosts(_ context.Context, _ *mcp.CallToolRequest, in ListHostsIn) (*mcp.CallToolResult, ListHostsOut, error) {
	out, err := s.doListHosts(in)
	s.record(toolListHosts, "", "", err)
	return nil, out, err
}

func (s *Server) doListHosts(in ListHostsIn) (ListHostsOut, error) {
	v, err := s.load(tierRead)

	if err != nil {
		return ListHostsOut{}, err
	}

	d := v.Data()
	q := strings.ToLower(strings.TrimSpace(in.Query))
	out := ListHostsOut{Hosts: []HostOut{}}

	for _, h := range shared(v) {
		if !inFolder(h, in.Folder) {
			continue
		}

		text := strings.ToLower(h.Title() + " " + h.Address + " " + strings.Join(h.Tags, " "))

		if q != "" && !strings.Contains(text, q) {
			continue
		}

		out.Hosts = append(out.Hosts, hostOut(h, d, v.Policy()))
	}

	return out, nil
}

type KeyOut struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Passphrase  bool   `json:"passphrase"`
	Certificate bool   `json:"certificate"`
}

type ListKeysOut struct {
	Keys []KeyOut `json:"keys"`
}

func fingerprint(k vault.Key) string {
	return sshx.Fingerprint(k.Public)
}

func (s *Server) listKeys(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListKeysOut, error) {
	out := ListKeysOut{Keys: []KeyOut{}}
	v, err := s.load(tierRead)

	if err == nil {
		for _, k := range v.Data().Keys {
			out.Keys = append(out.Keys, KeyOut{
				ID:          k.ID,
				Label:       k.Label,
				Type:        k.Type,
				Fingerprint: fingerprint(k),
				Passphrase:  k.Passphrase != "" || strings.Contains(k.Private, "ENCRYPTED"),
				Certificate: k.Certificate != "",
			})
		}
	}

	s.record(toolListKeys, "", "", err)
	return nil, out, err
}

type KeyIn struct {
	Key string `json:"key" jsonschema:"id or label of a key saved in Boreal"`
}

type PublicKeyOut struct {
	Label       string `json:"label"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"publicKey"`
}

func findKey(d vault.Data, ref string) (vault.Key, error) {
	ref = strings.TrimSpace(ref)
	var found []vault.Key

	for _, k := range d.Keys {
		if k.ID == ref {
			return k, nil
		}

		if strings.EqualFold(k.Label, ref) {
			found = append(found, k)
		}
	}

	switch len(found) {
	case 0:
		return vault.Key{}, fmt.Errorf("there's no key %q in Boreal", ref)
	case 1:
		return found[0], nil
	}

	return vault.Key{}, fmt.Errorf("several keys are called %q; use the id from list_keys", ref)
}

func (s *Server) publicKey(_ context.Context, _ *mcp.CallToolRequest, in KeyIn) (*mcp.CallToolResult, PublicKeyOut, error) {
	var out PublicKeyOut
	v, err := s.load(tierRead)

	if err == nil {
		var k vault.Key

		if k, err = findKey(v.Data(), in.Key); err == nil {
			out = PublicKeyOut{Label: k.Label, Fingerprint: fingerprint(k), PublicKey: strings.TrimSpace(k.Public)}
		}
	}

	s.record(toolPublicKey, "", in.Key, err)
	return nil, out, err
}

type SnippetOut struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Command string `json:"command"`
}

type ListSnippetsOut struct {
	Snippets []SnippetOut `json:"snippets"`
}

func (s *Server) listSnippets(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListSnippetsOut, error) {
	out := ListSnippetsOut{Snippets: []SnippetOut{}}
	v, err := s.load(tierRead)

	if err == nil {
		for _, sn := range v.Data().Snippets {
			out.Snippets = append(out.Snippets, SnippetOut{ID: sn.ID, Label: sn.Label, Command: sn.Command})
		}
	}

	s.record(toolListSnippets, "", "", err)
	return nil, out, err
}

type HostIn struct {
	Host string `json:"host" jsonschema:"id or label of a host from list_hosts"`
}

type AccessEntry struct {
	Fingerprint string   `json:"fingerprint"`
	Type        string   `json:"type"`
	Comment     string   `json:"comment,omitempty"`
	Options     []string `json:"options,omitempty"`
	KeyID       string   `json:"keyId,omitempty"`
	KeyLabel    string   `json:"keyLabel,omitempty"`
	Line        int      `json:"line"`
}

type AccessOut struct {
	Host    string        `json:"host"`
	Entries []AccessEntry `json:"entries"`
}

func findHost(v View, ref string) (vault.Host, error) {
	ref = strings.TrimSpace(ref)
	var found []vault.Host

	for _, h := range shared(v) {
		if h.ID == ref {
			return h, nil
		}

		if strings.EqualFold(h.Title(), ref) {
			found = append(found, h)
		}
	}

	switch len(found) {
	case 0:
		return vault.Host{}, fmt.Errorf("there's no host %q shared over MCP", ref)
	case 1:
		return found[0], nil
	}

	return vault.Host{}, fmt.Errorf("several hosts are called %q; use the id from list_hosts", ref)
}

func entries(content []byte, d vault.Data) []AccessEntry {
	byFP := map[string]vault.Key{}

	for _, k := range d.Keys {
		if fp := fingerprint(k); fp != "" {
			byFP[fp] = k
		}
	}

	out := []AccessEntry{}

	for _, e := range access.Parse(content) {
		a := AccessEntry{Fingerprint: e.Fingerprint, Type: e.Type, Comment: e.Comment, Options: e.Options, Line: e.Line}

		if k, ok := byFP[e.Fingerprint]; ok {
			a.KeyID, a.KeyLabel = k.ID, k.Label
		}

		out = append(out, a)
	}

	return out
}

func (s *Server) readAccess(ctx context.Context, v View, h vault.Host) (AccessOut, error) {
	files, closer, err := v.Files(ctx, h)

	if err != nil {
		return AccessOut{}, err
	}

	defer closer.Close()
	content, err := files.Read()

	if err != nil {
		return AccessOut{}, err
	}

	return AccessOut{Host: h.Title(), Entries: entries(content, v.Data())}, nil
}

func (s *Server) getAccess(ctx context.Context, _ *mcp.CallToolRequest, in HostIn) (*mcp.CallToolResult, AccessOut, error) {
	var out AccessOut
	v, err := s.load(tierRead)

	if err == nil {
		var h vault.Host

		if h, err = findHost(v, in.Host); err == nil {
			out, err = s.readAccess(ctx, v, h)
		}
	}

	s.record(toolGetAccess, in.Host, "", err)
	return nil, out, err
}

type AuditIn struct {
	Folder string `json:"folder,omitempty" jsonschema:"only hosts in this folder and its subfolders"`
}

type HostAccess struct {
	Host    string        `json:"host"`
	Entries []AccessEntry `json:"entries,omitempty"`
	Error   string        `json:"error,omitempty"`
}

type AuditOut struct {
	Hosts   []HostAccess `json:"hosts"`
	Skipped int          `json:"skipped,omitempty"`
}

func (s *Server) auditAccess(ctx context.Context, _ *mcp.CallToolRequest, in AuditIn) (*mcp.CallToolResult, AuditOut, error) {
	out := AuditOut{Hosts: []HostAccess{}}
	v, err := s.load(tierRead)

	if err == nil {
		for _, h := range shared(v) {
			if !inFolder(h, in.Folder) {
				continue
			}

			if len(out.Hosts) == maxAudit {
				out.Skipped++
				continue
			}

			a, herr := s.readAccess(ctx, v, h)
			row := HostAccess{Host: h.Title(), Entries: a.Entries}

			if herr != nil {
				row.Error = herr.Error()
			}

			out.Hosts = append(out.Hosts, row)
		}
	}

	s.record(toolAuditAccess, in.Folder, "", err)
	return nil, out, err
}

type GrantIn struct {
	Host         string              `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Key          string              `json:"key,omitempty" jsonschema:"id or label of a key saved in Boreal"`
	PublicKey    string              `json:"publicKey,omitempty" jsonschema:"a public key in authorized_keys format, for keys not saved in Boreal"`
	Label        string              `json:"label,omitempty" jsonschema:"who or what the key belongs to, written as the line's comment"`
	Restrictions access.Restrictions `json:"restrictions,omitzero" jsonschema:"optional limits on what the key may do"`
}

type GrantOut struct {
	Host        string `json:"host"`
	Fingerprint string `json:"fingerprint"`
	Changed     bool   `json:"changed"`
	Line        string `json:"line"`
}

func (s *Server) grantKey(d vault.Data, in GrantIn) (ssh.PublicKey, string, error) {
	switch {
	case in.Key != "" && in.PublicKey != "":
		return nil, "", errOneKey
	case in.Key != "":
		k, err := findKey(d, in.Key)

		if err != nil {
			return nil, "", err
		}

		pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.Public))

		if err != nil {
			return nil, "", fmt.Errorf("the public half of %q can't be read", k.Label)
		}

		return pub, cmpOr(in.Label, k.Label), nil
	case in.PublicKey != "":
		pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(in.PublicKey)))

		if err != nil {
			return nil, "", errors.New("publicKey isn't a valid SSH public key")
		}

		label := cmpOr(in.Label, comment)

		if strings.TrimSpace(label) == "" {
			return nil, "", errNoLabel
		}

		return pub, label, nil
	}

	return nil, "", errNoKey
}

func cmpOr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}

	return b
}

func (s *Server) grantAccess(ctx context.Context, _ *mcp.CallToolRequest, in GrantIn) (*mcp.CallToolResult, GrantOut, error) {
	out, err := s.doGrant(ctx, in)
	s.record(toolGrantAccess, in.Host, out.Fingerprint, err)
	return nil, out, err
}

func (s *Server) doGrant(ctx context.Context, in GrantIn) (GrantOut, error) {
	v, err := s.load(tierAccess)

	if err != nil {
		return GrantOut{}, err
	}

	h, err := findHost(v, in.Host)

	if err != nil {
		return GrantOut{}, err
	}

	pub, label, err := s.grantKey(v.Data(), in)

	if err != nil {
		return GrantOut{}, err
	}

	out := GrantOut{Host: h.Title(), Fingerprint: ssh.FingerprintSHA256(pub), Line: access.Line(pub, label, in.Restrictions)}

	if slices.Contains(protected(v, v.Data(), h), out.Fingerprint) {
		return out, errOwnKey
	}

	if err := in.Restrictions.Validate(s.now()); err != nil {
		return out, err
	}

	files, closer, err := v.Files(ctx, h)

	if err != nil {
		return out, err
	}

	defer closer.Close()
	current, err := files.Read()

	if err != nil {
		return out, err
	}

	next, changed, err := access.Grant(current, pub, label, in.Restrictions, s.now())

	if err != nil || !changed {
		return out, err
	}

	if err := files.Replace(current, next); err != nil {
		return out, err
	}

	out.Changed = true
	return out, nil
}

type RevokeIn struct {
	Host        string `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Key         string `json:"key,omitempty" jsonschema:"id or label of a key saved in Boreal"`
	Fingerprint string `json:"fingerprint,omitempty" jsonschema:"SHA256 fingerprint of the key to remove, as shown by get_access"`
}

type RevokeOut struct {
	Host        string `json:"host"`
	Fingerprint string `json:"fingerprint"`
	Removed     int    `json:"removed"`
}

func portOf(h vault.Host) int {
	if h.Port <= 0 {
		return defaultPort
	}

	return h.Port
}

func sameLogin(a, b vault.Host) bool {
	return a.ID == b.ID || strings.EqualFold(a.Address, b.Address) && portOf(a) == portOf(b) && a.Username == b.Username
}

func protected(v View, d vault.Data, h vault.Host) []string {
	var out []string

	for _, x := range d.Hosts {
		if !sameLogin(x, h) {
			continue
		}

		if x.Auth == vault.AuthKey && x.KeyID != "" {
			if i := slices.IndexFunc(d.Keys, func(k vault.Key) bool { return k.ID == x.KeyID }); i >= 0 {
				out = append(out, fingerprint(d.Keys[i]))
			}
		}

		switch {
		case x.AgentKey != "":
			out = append(out, x.AgentKey)
		case x.Auth == vault.AuthAgent:
			out = append(out, v.AgentKeys(x)...)
		}
	}

	return out
}

func (s *Server) revokeAccess(ctx context.Context, _ *mcp.CallToolRequest, in RevokeIn) (*mcp.CallToolResult, RevokeOut, error) {
	out, err := s.doRevoke(ctx, in)
	s.record(toolRevokeAccess, in.Host, out.Fingerprint, err)
	return nil, out, err
}

func (s *Server) doRevoke(ctx context.Context, in RevokeIn) (RevokeOut, error) {
	v, err := s.load(tierAccess)

	if err != nil {
		return RevokeOut{}, err
	}

	h, err := findHost(v, in.Host)

	if err != nil {
		return RevokeOut{}, err
	}

	d := v.Data()
	fp := strings.TrimSpace(in.Fingerprint)

	switch {
	case in.Key != "" && fp != "":
		return RevokeOut{}, errOneKey
	case in.Key != "":
		k, err := findKey(d, in.Key)

		if err != nil {
			return RevokeOut{}, err
		}

		fp = fingerprint(k)
	case fp == "":
		return RevokeOut{}, errRevokeArg
	}

	out := RevokeOut{Host: h.Title(), Fingerprint: fp}

	if slices.Contains(protected(v, d, h), fp) {
		return out, errLockout
	}

	files, closer, err := v.Files(ctx, h)

	if err != nil {
		return out, err
	}

	defer closer.Close()
	current, err := files.Read()

	if err != nil {
		return out, err
	}

	next, removed := access.Revoke(current, fp)

	if removed == 0 {
		return out, nil
	}

	if err := files.Replace(current, next); err != nil {
		return out, err
	}

	out.Removed = removed
	return out, nil
}
