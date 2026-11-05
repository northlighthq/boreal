package session

import (
	"sync"
	"time"
)

const (
	startupQuiet = 5 * time.Second
	echoQuiet    = 500 * time.Millisecond
)

// activity remembers when a session last printed something by itself. Output
// right after starting, typing or resizing is the shell answering, not news.
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

// LastOutput is when the session last printed something the user didn't
// directly cause. It's zero until that happens.
func (s *Session) LastOutput() time.Time {
	return s.activity.latest()
}
