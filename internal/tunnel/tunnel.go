package tunnel

import (
	"context"
	"sync"

	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/vault"
)

type State struct {
	Starting bool
	Tunnel   *sshx.Tunnel
	Client   *sshx.Client
	Err      error
}

type DialFunc func(ctx context.Context, r vault.Forward) (*sshx.Client, error)

type Notify func(title, message string)

type Manager struct {
	mu sync.Mutex
	m  map[string]*State

	dial    DialFunc
	changed func()
	info    Notify
	success Notify
	fail    Notify
}

func New(dial DialFunc) *Manager {
	noop := func(string, string) {}

	return &Manager{
		m:       map[string]*State{},
		dial:    dial,
		changed: func() {},
		info:    noop,
		success: noop,
		fail:    noop,
	}
}

func (t *Manager) OnChange(fn func()) {
	if fn != nil {
		t.changed = fn
	}
}

func (t *Manager) OnNotify(info, success, fail Notify) {
	if info != nil {
		t.info = info
	}

	if success != nil {
		t.success = success
	}

	if fail != nil {
		t.fail = fail
	}
}

func (t *Manager) State(id string) (State, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.m[id]

	if !ok {
		return State{}, false
	}

	return *s, true
}

func (t *Manager) Active() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := 0

	for _, s := range t.m {
		if s.Tunnel != nil {
			n++
		}
	}

	return n
}

func (t *Manager) Start(r vault.Forward) {
	t.mu.Lock()

	if s, ok := t.m[r.ID]; ok && (s.Starting || s.Tunnel != nil) {
		t.mu.Unlock()
		return
	}

	t.m[r.ID] = &State{Starting: true}
	t.mu.Unlock()
	t.changed()

	go t.open(r)
}

func (t *Manager) open(r vault.Forward) {
	client, err := t.dial(context.Background(), r)

	var tun *sshx.Tunnel

	if err == nil {
		tun, err = sshx.StartForward(client, r)

		if err != nil {
			client.Close()
		}
	}

	t.mu.Lock()
	t.m[r.ID] = &State{Tunnel: tun, Client: client, Err: err}
	t.mu.Unlock()

	if err != nil {
		t.fail("Couldn't open tunnel "+Label(r), err.Error())
		t.changed()
		return
	}

	t.success("Tunnel active", sshx.Describe(r))
	go t.watch(r.ID, tun, client)
	t.changed()
}

func (t *Manager) watch(id string, tun *sshx.Tunnel, client *sshx.Client) {
	done := make(chan struct{})

	go func() {
		client.Wait()
		close(done)
	}()

	select {
	case <-tun.Done():
	case <-done:
	}

	t.mu.Lock()
	s, ok := t.m[id]

	if !ok || s.Tunnel != tun {
		t.mu.Unlock()
		return
	}

	delete(t.m, id)
	t.mu.Unlock()
	tun.Close()
	client.Close()
	t.info("Tunnel stopped", sshx.Describe(tun.Rule))
	t.changed()
}

func (t *Manager) Stop(id string) {
	t.mu.Lock()
	s, ok := t.m[id]
	delete(t.m, id)
	t.mu.Unlock()

	if !ok {
		return
	}

	if s.Tunnel != nil {
		s.Tunnel.Close()
	}

	if s.Client != nil {
		s.Client.Close()
	}

	t.changed()
}

func (t *Manager) StopAll() {
	t.mu.Lock()
	ids := make([]string, 0, len(t.m))

	for id := range t.m {
		ids = append(ids, id)
	}

	t.mu.Unlock()

	for _, id := range ids {
		t.Stop(id)
	}
}

func Label(r vault.Forward) string {
	if r.Label != "" {
		return r.Label
	}

	return sshx.Describe(r)
}
