package app

import (
	"regexp"
	"slices"
	"time"

	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/platform/sound"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/ui/termview"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	triggerCooldown = 3 * time.Second
	triggerLineMax  = 200
)

var triggerColors = map[string]vt.Color{
	vault.TriggerRed:     vt.Indexed(9),
	vault.TriggerYellow:  vt.Indexed(11),
	vault.TriggerGreen:   vt.Indexed(10),
	vault.TriggerBlue:    vt.Indexed(12),
	vault.TriggerMagenta: vt.Indexed(13),
	vault.TriggerCyan:    vt.Indexed(14),
}

type compiledTrigger struct {
	vault.Trigger
	re *regexp.Regexp
}

type triggerKey struct {
	pane *Pane
	id   string
}

func compileTriggers(list []vault.Trigger) []compiledTrigger {
	out := make([]compiledTrigger, 0, len(list))

	for _, t := range list {
		re, err := t.Compile()

		if err != nil {
			continue
		}

		out = append(out, compiledTrigger{Trigger: t, re: re})
	}

	return out
}

func triggerApplies(t vault.Trigger, p *Pane) bool {
	switch t.HostID {
	case "":
		return true
	case vault.TriggerLocal:
		return p.sess.Kind == session.Local
	}

	return p.sess.Kind == session.SSH && p.sess.HostID == t.HostID
}

func (a *App) applyTriggers() {
	list := a.vault.Snapshot().Triggers

	if slices.Equal(list, a.triggerSrc) {
		return
	}

	a.triggerSrc = list
	compiled := compileTriggers(list)

	a.mu.Lock()
	a.triggers = compiled
	a.mu.Unlock()

	for _, t := range a.ws.tabs {
		for _, p := range t.root.panes(nil) {
			a.applyTriggersTo(p)
		}
	}
}

func (a *App) applyTriggersTo(p *Pane) {
	a.mu.Lock()
	list := a.triggers
	a.mu.Unlock()

	var rules []session.Rule
	var marks []termview.Mark

	for _, t := range list {
		if !triggerApplies(t.Trigger, p) {
			continue
		}

		if t.Notify || t.Sound {
			rules = append(rules, session.Rule{ID: t.ID, Re: t.re})
		}

		if c, ok := triggerColors[t.Color]; ok {
			marks = append(marks, termview.Mark{Re: t.re, Color: c})
		}
	}

	p.sess.SetTriggers(rules, func(r session.Rule, line string) { a.fireTrigger(p, r.ID, line) })
	p.view.SetMarks(marks)
}

func (a *App) fireTrigger(p *Pane, id, line string) {
	t, ok := a.claimTrigger(p, id, time.Now())

	if !ok {
		return
	}

	if t.Sound {
		sound.Play()
	}

	if !t.Notify {
		return
	}

	if r := a.redactor(); r != nil {
		line = r(line)
	}

	if runes := []rune(line); len(runes) > triggerLineMax {
		line = string(runes[:triggerLineMax]) + "…"
	}

	a.toasts.Notice(p.sess.Title(), line, "Show", func() {
		a.mu.Lock()
		a.revealing = p
		a.mu.Unlock()
		a.Invalidate()
	})
}

func (a *App) revealPending() {
	a.mu.Lock()
	p := a.revealing
	a.revealing = nil
	a.mu.Unlock()

	if p == nil || a.ws.tabOf(p) == nil {
		return
	}

	a.focusPane(p)
	a.ws.focusActive()
}

func (a *App) claimTrigger(p *Pane, id string, now time.Time) (vault.Trigger, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for k, at := range a.triggerFired {
		if now.Sub(at) >= triggerCooldown {
			delete(a.triggerFired, k)
		}
	}

	key := triggerKey{pane: p, id: id}

	if _, ok := a.triggerFired[key]; ok {
		return vault.Trigger{}, false
	}

	for _, t := range a.triggers {
		if t.ID != id {
			continue
		}

		if a.triggerFired == nil {
			a.triggerFired = map[triggerKey]time.Time{}
		}

		a.triggerFired[key] = now
		return t.Trigger, true
	}

	return vault.Trigger{}, false
}
