package tunnel

import (
	"context"
	"sync"

	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

type State struct {
	Starting bool
	Shared   bool
	Tunnel   *sshx.Tunnel
	Client   *sshx.Client
	Err      error
}

type DialFunc func(ctx context.Context, r vault.Forward) (*sshx.Client, error)

type Notify func(title, message string)

type Manager struct {
	mu      sync.Mutex
	m       map[string]*State
	dropped map[string]vault.Forward

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
		dropped: map[string]vault.Forward{},
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
	if t.claim(r.ID) {
		go t.open(r, nil)
	}
}

func (t *Manager) StartOn(r vault.Forward, c *sshx.Client) {
	if t.claim(r.ID) {
		go t.open(r, c)
	}
}

func (t *Manager) claim(id string) bool {
	t.mu.Lock()

	if s, ok := t.m[id]; ok && (s.Starting || s.Tunnel != nil) {
		t.mu.Unlock()
		return false
	}

	t.m[id] = &State{Starting: true}
	t.mu.Unlock()
	t.changed()
	return true
}

func (t *Manager) open(r vault.Forward, shared *sshx.Client) {
	client, err := shared, error(nil)

	if client == nil {
		client, err = t.dial(context.Background(), r)
	}

	var tun *sshx.Tunnel

	if err == nil {
		tun, err = sshx.StartForward(client, r)

		if err != nil && shared == nil {
			client.Close()
		}
	}

	t.mu.Lock()
	t.m[r.ID] = &State{Tunnel: tun, Client: client, Shared: shared != nil, Err: err}
	t.mu.Unlock()

	if err != nil {
		t.fail("Couldn't open tunnel "+Label(r), err.Error())
		t.changed()
		return
	}

	if shared == nil {
		t.success("Tunnel active", sshx.Describe(r))
	}

	go t.watch(r.ID, tun, client, shared != nil)
	t.changed()
}

func (t *Manager) watch(id string, tun *sshx.Tunnel, client *sshx.Client, shared bool) {
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

	if !shared {
		client.Close()
		t.mu.Lock()
		t.dropped[id] = tun.Rule
		t.mu.Unlock()
		t.info("Tunnel stopped", sshx.Describe(tun.Rule))
	}

	t.changed()
}

func (t *Manager) Stop(id string) {
	t.mu.Lock()
	s, ok := t.m[id]
	delete(t.m, id)
	delete(t.dropped, id)
	t.mu.Unlock()

	if !ok {
		return
	}

	if s.Tunnel != nil {
		s.Tunnel.Close()
	}

	if s.Client != nil && !s.Shared {
		s.Client.Close()
	}

	t.changed()
}

func (t *Manager) TakeDropped(hostID string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var ids []string

	for id, r := range t.dropped {
		if r.HostID == hostID {
			ids = append(ids, id)
			delete(t.dropped, id)
		}
	}

	return ids
}

func (t *Manager) Running() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	ids := make([]string, 0, len(t.m))

	for id, s := range t.m {
		if (s.Tunnel != nil || s.Starting) && !s.Shared {
			ids = append(ids, id)
		}
	}

	return ids
}

func (t *Manager) Traffic(hostID string) (int, sshx.Traffic) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var sum sshx.Traffic
	n := 0

	for _, s := range t.m {
		if s.Tunnel == nil || (hostID != "" && s.Tunnel.Rule.HostID != hostID) {
			continue
		}

		n++
		sum.Add(s.Tunnel.Traffic())
	}

	return n, sum
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
