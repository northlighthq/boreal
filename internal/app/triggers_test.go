package app

import (
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/vault"
)

func TestTriggerScope(t *testing.T) {
	local := &Pane{sess: &session.Session{Kind: session.Local}}
	web := &Pane{sess: &session.Session{Kind: session.SSH, HostID: "web"}}
	db := &Pane{sess: &session.Session{Kind: session.SSH, HostID: "db"}}

	cases := []struct {
		host string
		want []*Pane
	}{
		{"", []*Pane{local, web, db}},
		{vault.TriggerLocal, []*Pane{local}},
		{"web", []*Pane{web}},
	}

	for _, c := range cases {
		for _, p := range []*Pane{local, web, db} {
			want := false

			for _, w := range c.want {
				want = want || w == p
			}

			if got := triggerApplies(vault.Trigger{HostID: c.host}, p); got != want {
				t.Fatalf("scope %q on %v/%q: got %v, want %v", c.host, p.sess.Kind, p.sess.HostID, got, want)
			}
		}
	}
}

func TestCompileTriggersSkipsBadPatterns(t *testing.T) {
	list := compileTriggers([]vault.Trigger{{ID: "a", Pattern: "error"}, {ID: "b", Pattern: "("}, {ID: "c", Pattern: "  "}})

	if len(list) != 1 || list[0].ID != "a" || !list[0].re.MatchString("ERROR: x") {
		t.Fatalf("only the valid pattern should stay and ignore case, got %+v", list)
	}
}

func TestTriggerCooldown(t *testing.T) {
	a := &App{triggers: compileTriggers([]vault.Trigger{{ID: "a", Pattern: "x", Notify: true}})}
	p, q := &Pane{}, &Pane{}
	now := time.Unix(1000, 0)

	if _, ok := a.claimTrigger(p, "a", now); !ok {
		t.Fatal("the first match should fire")
	}

	if _, ok := a.claimTrigger(p, "a", now.Add(time.Second)); ok {
		t.Fatal("a match right after should wait for the cooldown")
	}

	if _, ok := a.claimTrigger(q, "a", now.Add(time.Second)); !ok {
		t.Fatal("another pane has its own cooldown")
	}

	if _, ok := a.claimTrigger(p, "a", now.Add(triggerCooldown)); !ok {
		t.Fatal("after the cooldown it should fire again")
	}

	if _, ok := a.claimTrigger(p, "gone", now.Add(time.Hour)); ok {
		t.Fatal("a removed trigger must not fire")
	}
}
