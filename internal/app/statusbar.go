package app

import (
	"sync"
	"time"

	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/metrics"
)

const statsEvery = 3 * time.Second

type paneStats struct {
	mu      sync.Mutex
	st      sshx.Stats
	hist    metrics.History
	have    bool
	err     error
	running bool
	seen    time.Time
}

func (s *paneStats) get() (sshx.Stats, metrics.History, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st, s.hist.Clone(), s.have, s.err
}

func (a *App) watchStats(p *Pane) {
	ps := &p.stats
	ps.mu.Lock()
	ps.seen = time.Now()

	if ps.running {
		ps.mu.Unlock()
		return
	}

	ps.running = true
	ps.mu.Unlock()

	go func() {
		var sm sshx.Sampler

		for {
			c := p.sess.Client()

			if c == nil {
				ps.stop()
				return
			}

			st, err := sm.Sample(c)
			ps.mu.Lock()

			if err == nil {
				ps.st, ps.have = st, true
				ps.hist.Push(st)
			}

			ps.err = err
			ps.mu.Unlock()
			a.Invalidate()

			if err == sshx.ErrNoStats {
				ps.stop()
				return
			}

			time.Sleep(statsEvery)

			if ps.stopIfHidden() {
				return
			}
		}
	}()
}

func (s *paneStats) stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *paneStats) stopIfHidden() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.seen) <= 2*statsEvery {
		return false
	}

	s.running = false
	return true
}

func (c *chrome) statusBar(a *App, gtx C) D {
	p := a.ws.activePane()

	if p == nil || p.sess.Kind != session.SSH {
		return D{}
	}

	if st, _ := p.sess.State(); st != session.Connected {
		return D{}
	}

	a.watchStats(p)

	s, hist, have, err := p.stats.get()
	return c.metrics.Layout(gtx, a.kit, s, hist, have, err)
}
