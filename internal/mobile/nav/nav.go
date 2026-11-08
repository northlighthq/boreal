package nav

import (
	"time"

	"gioui.org/layout"
)

type Screen interface {
	Title() string
	Layout(gtx layout.Context) layout.Dimensions
}

type Stack struct {
	screens []Screen
	moved   time.Time
	back    bool
}

func NewStack(root Screen) *Stack {
	return &Stack{screens: []Screen{root}}
}

func (s *Stack) Top() Screen {
	return s.screens[len(s.screens)-1]
}

func (s *Stack) Depth() int {
	return len(s.screens)
}

func (s *Stack) CanPop() bool {
	return len(s.screens) > 1
}

func (s *Stack) Push(sc Screen, now time.Time) {
	s.screens = append(s.screens, sc)
	s.moved, s.back = now, false
}

func (s *Stack) Pop(now time.Time) bool {
	if !s.CanPop() {
		return false
	}

	s.screens[len(s.screens)-1] = nil
	s.screens = s.screens[:len(s.screens)-1]
	s.moved, s.back = now, true
	return true
}

func (s *Stack) PopToRoot(now time.Time) bool {
	if !s.CanPop() {
		return false
	}

	clear(s.screens[1:])
	s.screens = s.screens[:1]
	s.moved, s.back = now, true
	return true
}

func (s *Stack) Transition(now time.Time, d time.Duration) (progress float32, back bool) {
	if s.moved.IsZero() || d <= 0 {
		return 1, s.back
	}

	p := float32(now.Sub(s.moved)) / float32(d)
	return min(max(p, 0), 1), s.back
}
