package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

const (
	toolListTunnels   = "list_tunnels"
	toolStartTunnel   = "start_tunnel"
	toolStopTunnel    = "stop_tunnel"
	defaultTunnelTime = 60
	maxTunnelTime     = 8 * 60
	tunnelRecheck     = 10 * time.Second
)

var (
	errNoTunnels   = errors.New("starting tunnels isn't allowed. It can be allowed in " + settingsPath)
	errOpenBind    = errors.New("this tunnel listens on an address other machines can reach, so it can only be started from Boreal")
	errNotRunning  = errors.New("this tunnel isn't running in this MCP session")
	errTunnelEmpty = errors.New("give the id or label of a tunnel from list_tunnels")
)

type Tunnel interface {
	Close() error
	Done() <-chan struct{}
	Traffic() sshx.Traffic
}

type running struct {
	t     Tunnel
	until time.Time
	timer *time.Timer
}

type tunnels struct {
	mu      sync.Mutex
	m       map[string]*running
	minute  time.Duration
	recheck time.Duration
	quit    chan struct{}
	once    sync.Once
	watch   bool
}

type TunnelOut struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Host        string     `json:"host"`
	Kind        string     `json:"kind"`
	Route       string     `json:"route"`
	Running     bool       `json:"running"`
	Until       *time.Time `json:"until,omitempty"`
	Connections int        `json:"connections,omitempty"`
	Startable   bool       `json:"startable"`
}

type ListTunnelsOut struct {
	Tunnels []TunnelOut `json:"tunnels"`
}

type StartTunnelIn struct {
	Tunnel  string `json:"tunnel" jsonschema:"id or label of a tunnel from list_tunnels"`
	Minutes int    `json:"minutes,omitempty" jsonschema:"minutes before Boreal stops the tunnel, from 1 to 480. 60 by default"`
}

type TunnelIn struct {
	Tunnel string `json:"tunnel" jsonschema:"id or label of a tunnel from list_tunnels"`
}

func (s *Server) addTunnels(srv *mcp.Server) {
	closed, destructive := false, false
	reads := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}
	changes := &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: &destructive, OpenWorldHint: &closed}

	mcp.AddTool(srv, &mcp.Tool{Name: toolListTunnels, Description: "List the tunnels (port forwards) saved in Boreal for hosts shared over MCP, and which ones this MCP session is running.", Annotations: reads}, s.listTunnels)
	mcp.AddTool(srv, &mcp.Tool{Name: toolStartTunnel, Description: fmt.Sprintf("Start a tunnel saved in Boreal. It runs while this MCP session is open and stops after the given minutes (%d by default, %d at most). Only tunnels that listen on this computer's loopback address can be started.", defaultTunnelTime, maxTunnelTime), Annotations: changes}, s.startTunnel)
	mcp.AddTool(srv, &mcp.Tool{Name: toolStopTunnel, Description: "Stop a tunnel this MCP session started.", Annotations: changes}, s.stopTunnel)
}

func startable(f vault.Forward) bool {
	return sshx.Loopback(f.BindAddr)
}

func (s *Server) sharedForwards(v View) []vault.Forward {
	hosts := map[string]bool{}

	for _, h := range shared(v) {
		hosts[h.ID] = true
	}

	var out []vault.Forward

	for _, f := range v.Data().Forwards {
		if hosts[f.HostID] {
			out = append(out, f)
		}
	}

	return out
}

func findForward(list []vault.Forward, ref string) (vault.Forward, error) {
	ref = strings.TrimSpace(ref)

	if ref == "" {
		return vault.Forward{}, errTunnelEmpty
	}

	var found []vault.Forward

	for _, f := range list {
		if f.ID == ref {
			return f, nil
		}

		if strings.EqualFold(f.Label, ref) || strings.EqualFold(sshx.Describe(f), ref) {
			found = append(found, f)
		}
	}

	switch len(found) {
	case 0:
		return vault.Forward{}, fmt.Errorf("there's no tunnel %q on a host shared over MCP", ref)
	case 1:
		return found[0], nil
	}

	return vault.Forward{}, fmt.Errorf("several tunnels are called %q; use the id from list_tunnels", ref)
}

func (s *Server) tunnelOut(v View, f vault.Forward) TunnelOut {
	o := TunnelOut{ID: f.ID, Label: f.Label, Kind: f.Kind, Route: sshx.Describe(f), Startable: startable(f)}

	if h, ok := hostByID(v, f.HostID); ok {
		o.Host = h.Title()
	}

	s.tun.mu.Lock()
	defer s.tun.mu.Unlock()

	if r, ok := s.tun.m[f.ID]; ok {
		until := r.until.UTC()
		o.Running, o.Until, o.Connections = true, &until, r.t.Traffic().Conns
	}

	return o
}

func hostByID(v View, id string) (vault.Host, bool) {
	i := slices.IndexFunc(v.Data().Hosts, func(h vault.Host) bool { return h.ID == id })

	if i < 0 {
		return vault.Host{}, false
	}

	return v.Data().Hosts[i], true
}

func (s *Server) recordTunnel(tool string, f vault.Forward, ref string, err error) {
	r := Record{Time: s.now(), Tool: tool, Tunnel: ref, Outcome: OutcomeOK}

	if f.ID != "" {
		r.Tunnel = sshx.Describe(f)
	}

	if err != nil {
		r.Outcome, r.Error = OutcomeFail, err.Error()
	}

	s.log.Record(r)
}

func (s *Server) listTunnels(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListTunnelsOut, error) {
	v, err := s.load(tierTunnels)

	if err != nil {
		return nil, ListTunnelsOut{}, err
	}

	out := ListTunnelsOut{Tunnels: []TunnelOut{}}

	for _, f := range s.sharedForwards(v) {
		out.Tunnels = append(out.Tunnels, s.tunnelOut(v, f))
	}

	return nil, out, nil
}

func tunnelTime(minutes int) int {
	switch {
	case minutes <= 0:
		return defaultTunnelTime
	case minutes > maxTunnelTime:
		return maxTunnelTime
	}

	return minutes
}

func (s *Server) startTunnel(ctx context.Context, _ *mcp.CallToolRequest, in StartTunnelIn) (*mcp.CallToolResult, TunnelOut, error) {
	var f vault.Forward
	out, err := s.start(ctx, in, &f)
	s.recordTunnel(toolStartTunnel, f, in.Tunnel, err)
	return nil, out, err
}

func (s *Server) start(ctx context.Context, in StartTunnelIn, f *vault.Forward) (TunnelOut, error) {
	v, err := s.load(tierTunnels)

	if err != nil {
		return TunnelOut{}, err
	}

	if *f, err = findForward(s.sharedForwards(v), in.Tunnel); err != nil {
		return TunnelOut{}, err
	}

	if !startable(*f) {
		return TunnelOut{}, errOpenBind
	}

	h, err := findHost(v, f.HostID)

	if err != nil {
		return TunnelOut{}, err
	}

	s.tun.mu.Lock()
	_, already := s.tun.m[f.ID]
	s.tun.mu.Unlock()

	if already {
		return s.tunnelOut(v, *f), nil
	}

	t, err := v.Forward(ctx, h, *f)

	if err != nil {
		return TunnelOut{}, err
	}

	now := s.now()
	life := time.Duration(tunnelTime(in.Minutes)) * s.tun.minute
	r := &running{t: t, until: now.Add(life)}
	id := f.ID

	s.tun.mu.Lock()

	if s.tun.m == nil {
		s.tun.m = map[string]*running{}
	}

	if _, raced := s.tun.m[id]; raced {
		s.tun.mu.Unlock()
		t.Close()
		return s.tunnelOut(v, *f), nil
	}

	s.tun.m[id] = r
	r.timer = time.AfterFunc(life, func() { s.drop(id, r) })
	watch := !s.tun.watch
	s.tun.watch = true
	s.tun.mu.Unlock()

	go func() {
		<-t.Done()
		s.drop(id, r)
	}()

	if watch {
		go s.police()
	}

	return s.tunnelOut(v, *f), nil
}

func (s *Server) drop(id string, r *running) {
	s.tun.mu.Lock()

	if cur, ok := s.tun.m[id]; !ok || cur != r {
		s.tun.mu.Unlock()
		return
	}

	delete(s.tun.m, id)
	s.tun.mu.Unlock()
	r.timer.Stop()
	r.t.Close()
}

func (s *Server) stopTunnel(_ context.Context, _ *mcp.CallToolRequest, in TunnelIn) (*mcp.CallToolResult, TunnelOut, error) {
	var f vault.Forward
	out, err := s.stop(in, &f)
	s.recordTunnel(toolStopTunnel, f, in.Tunnel, err)
	return nil, out, err
}

func (s *Server) stop(in TunnelIn, f *vault.Forward) (TunnelOut, error) {
	v, err := s.load(tierTunnels)

	if err != nil {
		return TunnelOut{}, err
	}

	if *f, err = findForward(s.sharedForwards(v), in.Tunnel); err != nil {
		return TunnelOut{}, err
	}

	s.tun.mu.Lock()
	r, ok := s.tun.m[f.ID]
	s.tun.mu.Unlock()

	if !ok {
		return TunnelOut{}, errNotRunning
	}

	s.drop(f.ID, r)
	return s.tunnelOut(v, *f), nil
}

func (s *Server) allowed() map[string]bool {
	v, err := s.backend()

	if err != nil {
		return nil
	}

	p := v.Policy()

	if !p.Enabled || !p.Read || !p.Tunnels || !p.Scoped() {
		return nil
	}

	ids := map[string]bool{}

	for _, f := range s.sharedForwards(v) {
		if startable(f) {
			ids[f.ID] = true
		}
	}

	return ids
}

func (s *Server) police() {
	tick := time.NewTicker(s.tun.recheck)
	defer tick.Stop()

	for {
		select {
		case <-s.tun.quit:
			return
		case <-tick.C:
		}

		ok := s.allowed()
		s.tun.mu.Lock()
		var gone []string

		for id := range s.tun.m {
			if !ok[id] {
				gone = append(gone, id)
			}
		}

		rs := make([]*running, len(gone))

		for i, id := range gone {
			rs[i] = s.tun.m[id]
		}

		s.tun.mu.Unlock()

		for i, id := range gone {
			s.drop(id, rs[i])
		}
	}
}

func (s *Server) Close() {
	s.tun.once.Do(func() { close(s.tun.quit) })
	s.tun.mu.Lock()
	all := make(map[string]*running, len(s.tun.m))

	for id, r := range s.tun.m {
		all[id] = r
	}

	s.tun.mu.Unlock()

	for id, r := range all {
		s.drop(id, r)
	}
}
