package nav

import (
	"testing"
	"time"

	"gioui.org/layout"
)

type page string

func (p page) Title() string { return string(p) }

func (p page) Layout(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }

func TestStack(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewStack(page("hosts"))

	if s.CanPop() || s.Pop(now) {
		t.Fatal("the root screen must not pop")
	}

	s.Push(page("editor"), now)
	s.Push(page("keys"), now)

	if s.Depth() != 3 || s.Top().Title() != "keys" {
		t.Fatalf("got depth %d and top %q", s.Depth(), s.Top().Title())
	}

	if !s.Pop(now) || s.Top().Title() != "editor" {
		t.Fatalf("pop should go back to the editor, got %q", s.Top().Title())
	}

	if !s.PopToRoot(now) || s.Depth() != 1 || s.Top().Title() != "hosts" {
		t.Fatalf("pop to root should leave only the hosts screen, got %d", s.Depth())
	}

	if s.PopToRoot(now) {
		t.Fatal("pop to root at the root must report no change")
	}
}

func TestTransition(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewStack(page("hosts"))

	if p, _ := s.Transition(now, time.Second); p != 1 {
		t.Fatalf("a stack that never moved is at rest, got %v", p)
	}

	s.Push(page("editor"), now)

	if p, back := s.Transition(now.Add(250*time.Millisecond), time.Second); p != 0.25 || back {
		t.Fatalf("push halfway: got %v back=%v", p, back)
	}

	s.Pop(now)

	if p, back := s.Transition(now.Add(2*time.Second), time.Second); p != 1 || !back {
		t.Fatalf("pop finished: got %v back=%v", p, back)
	}
}
