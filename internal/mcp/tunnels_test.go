package mcp

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

var withTunnels = vault.MCPPolicy{Enabled: true, Read: true, Tunnels: true, Folders: []string{"Production"}}

type fakeTunnel struct {
	done chan struct{}
	once sync.Once
}

func (t *fakeTunnel) Close() error {
	t.once.Do(func() { close(t.done) })
	return nil
}

func (t *fakeTunnel) Done() <-chan struct{} { return t.done }
func (t *fakeTunnel) Traffic() sshx.Traffic { return sshx.Traffic{Conns: 2} }

func (t *fakeTunnel) closed() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

func (v *fakeView) Forward(_ context.Context, h vault.Host, f vault.Forward) (Tunnel, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	t := &fakeTunnel{done: make(chan struct{})}
	v.forwards = append(v.forwards, h.ID+"/"+f.ID)
	v.open = append(v.open, t)
	return t, nil
}

func (v *fakeView) started() []*fakeTunnel {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]*fakeTunnel(nil), v.open...)
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)

	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal(what)
		}

		time.Sleep(5 * time.Millisecond)
	}
}

func TestTunnelsNeedTheirOwnSwitch(t *testing.T) {
	f := setup(t, vault.MCPPolicy{Enabled: true, Read: true, Access: true, Execute: true, Files: true, WriteFiles: true, AllHosts: true})

	calls := map[string]map[string]any{
		toolListTunnels: {},
		toolStartTunnel: {"tunnel": "pg"},
		toolStopTunnel:  {"tunnel": "pg"},
	}

	for tool, args := range calls {
		if text, isErr := f.call(t, tool, args); !isErr || !strings.Contains(text, "tunnels isn't allowed") {
			t.Fatalf("%s without Tunnels: %v %q", tool, isErr, text)
		}
	}

	if len(f.view.started()) != 0 {
		t.Fatal("a tunnel started without permission")
	}
}

func TestListTunnelsOnlyShowsSharedHosts(t *testing.T) {
	f := setup(t, withTunnels)

	text, isErr := f.call(t, toolListTunnels, map[string]any{})

	if isErr || !strings.Contains(text, `"id":"pg"`) || !strings.Contains(text, `"id":"lan"`) || strings.Contains(text, "dbf") {
		t.Fatalf("list = %v %q", isErr, text)
	}

	if !strings.Contains(text, `"label":"Public web","route":"0.0.0.0:8080 → localhost:80","running":false,"startable":false`) {
		t.Fatalf("an open tunnel should be marked as not startable: %q", text)
	}
}

func TestStartAndStopATunnel(t *testing.T) {
	f := setup(t, withTunnels)

	text, isErr := f.call(t, toolStartTunnel, map[string]any{"tunnel": "postgres", "minutes": 30})

	if isErr || !strings.Contains(text, `"running":true`) || !strings.Contains(text, `"until":"2026-11-20T12:30:00Z"`) {
		t.Fatalf("start = %v %q", isErr, text)
	}

	if _, isErr := f.call(t, toolStartTunnel, map[string]any{"tunnel": "pg"}); isErr || len(f.view.started()) != 1 {
		t.Fatalf("starting it again opened another tunnel: %v", f.view.forwards)
	}

	if text, _ := f.call(t, toolListTunnels, map[string]any{}); !strings.Contains(text, `"connections":2`) {
		t.Fatalf("list while running = %q", text)
	}

	if text, isErr := f.call(t, toolStopTunnel, map[string]any{"tunnel": "pg"}); isErr || !strings.Contains(text, `"running":false`) {
		t.Fatalf("stop = %v %q", isErr, text)
	}

	if !f.view.started()[0].closed() {
		t.Fatal("stopping didn't close the tunnel")
	}

	if text, isErr := f.call(t, toolStopTunnel, map[string]any{"tunnel": "pg"}); !isErr || !strings.Contains(text, "isn't running") {
		t.Fatalf("stop twice = %v %q", isErr, text)
	}

	last := f.log.records[len(f.log.records)-1]

	if last.Tool != toolStopTunnel || last.Tunnel != "127.0.0.1:15432 → localhost:5432" {
		t.Fatalf("record = %+v", last)
	}
}

func TestOpenAndUnsharedTunnelsDontStart(t *testing.T) {
	f := setup(t, withTunnels)

	if text, isErr := f.call(t, toolStartTunnel, map[string]any{"tunnel": "lan"}); !isErr || !strings.Contains(text, "other machines") {
		t.Fatalf("open bind = %v %q", isErr, text)
	}

	if text, isErr := f.call(t, toolStartTunnel, map[string]any{"tunnel": "dbf"}); !isErr || !strings.Contains(text, "no tunnel") {
		t.Fatalf("unshared host = %v %q", isErr, text)
	}

	if len(f.view.started()) != 0 {
		t.Fatalf("tunnels started: %v", f.view.forwards)
	}
}

func TestTunnelStopsAfterItsTime(t *testing.T) {
	f := setup(t, withTunnels)
	f.srv.tun.minute = 20 * time.Millisecond

	f.call(t, toolStartTunnel, map[string]any{"tunnel": "pg", "minutes": 1})
	eventually(t, "the tunnel didn't stop after its time", func() bool { return f.view.started()[0].closed() })

	if text, _ := f.call(t, toolListTunnels, map[string]any{}); strings.Contains(text, `"running":true`) {
		t.Fatalf("still listed as running: %q", text)
	}
}

func TestTunnelStopsWhenThePermissionIsTurnedOff(t *testing.T) {
	f := setup(t, withTunnels)
	f.srv.tun.recheck = 10 * time.Millisecond

	f.call(t, toolStartTunnel, map[string]any{"tunnel": "pg"})
	time.Sleep(30 * time.Millisecond)

	if f.view.started()[0].closed() {
		t.Fatal("the tunnel stopped while still allowed")
	}

	p := withTunnels
	p.Tunnels = false
	f.view.setPolicy(p)
	eventually(t, "the tunnel kept running after the permission was turned off", func() bool { return f.view.started()[0].closed() })
}

func TestTunnelThatDiesIsForgotten(t *testing.T) {
	f := setup(t, withTunnels)
	f.call(t, toolStartTunnel, map[string]any{"tunnel": "pg"})
	f.view.started()[0].Close()

	eventually(t, "a dead tunnel is still listed as running", func() bool {
		text, _ := f.call(t, toolListTunnels, map[string]any{})
		return !strings.Contains(text, `"running":true`)
	})

	f.call(t, toolStartTunnel, map[string]any{"tunnel": "pg"})

	if len(f.view.started()) != 2 {
		t.Fatal("a dead tunnel couldn't be started again")
	}
}

func TestCloseStopsEveryTunnel(t *testing.T) {
	f := setup(t, withTunnels)
	f.call(t, toolStartTunnel, map[string]any{"tunnel": "pg"})
	f.srv.Close()

	if !f.view.started()[0].closed() {
		t.Fatal("Close left a tunnel running")
	}
}
