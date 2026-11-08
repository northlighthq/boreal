package session

import (
	"sync"
	"time"
)

const (
	startupQuiet = 5 * time.Second
	echoQuiet    = 500 * time.Millisecond
)

type activity struct {
	mu    sync.Mutex
	quiet time.Time
	last  time.Time
}

func (a *activity) hush(now time.Time, d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if until := now.Add(d); until.After(a.quiet) {
		a.quiet = until
	}
}

func (a *activity) note(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if now.Before(a.quiet) {
		return
	}

	a.last = now
}

func (a *activity) latest() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

func (s *Session) LastOutput() time.Time {
	return s.activity.latest()
}
