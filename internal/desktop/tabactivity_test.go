package desktop

import (
	"testing"
	"time"
)

func TestMarkSeenTabs(t *testing.T) {
	now := time.Unix(1000, 0)
	active, background := &Tab{}, &Tab{}
	a := &App{}
	a.ws.tabs = []*Tab{active, background}
	a.settings.TabActivity = true

	a.markSeenTabs(now)

	if !active.seen.Equal(now) {
		t.Fatal("the open tab should count as seen")
	}

	if !background.seen.IsZero() {
		t.Fatal("a background tab should stay unseen")
	}

	a.settings.TabActivity = false
	a.markSeenTabs(now)

	if !background.seen.Equal(now) {
		t.Fatal("with the setting off, nothing should build up to show later")
	}
}
