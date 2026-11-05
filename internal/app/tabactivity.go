package app

import (
	"time"

	"github.com/northlighthq/boreal/internal/ui/kit"
)

func (t *Tab) lastOutput() time.Time {
	var last time.Time

	for _, p := range t.root.panes(nil) {
		if at := p.sess.LastOutput(); at.After(last) {
			last = at
		}
	}

	return last
}

func (t *Tab) unread() bool {
	return t.lastOutput().After(t.seen)
}

func (a *App) markSeenTabs(now time.Time) {
	for i, t := range a.ws.tabs {
		if i == a.ws.active || !a.settings.TabActivity {
			t.seen = now
		}
	}
}

func (c *chrome) tabUnread(a *App, gtx C, t *Tab) float32 {
	return a.kit.Animated(gtx, kit.Key(&t.seen, 0), kit.B2F(t.unread()), kit.Normal)
}
