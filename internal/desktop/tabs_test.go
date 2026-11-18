package desktop

import "testing"

func appWithTabs(n, active int) *App {
	a := &App{}

	for range n {
		a.ws.tabs = append(a.ws.tabs, &Tab{})
	}

	a.ws.active = active
	return a
}

func TestTabWindowKeepsPlace(t *testing.T) {
	a := appWithTabs(20, 0)
	c := &chrome{}

	if got := c.tabWindow(a, 20, 5); got != 0 {
		t.Fatalf("first = %d, want 0", got)
	}

	a.ws.active = 3

	if got := c.tabWindow(a, 20, 5); got != 0 {
		t.Fatalf("a visible tab should not move the window, first = %d", got)
	}

	a.ws.active = 9

	if got := c.tabWindow(a, 20, 5); got != 5 {
		t.Fatalf("first = %d, want 5 so tab 9 is the last one shown", got)
	}

	a.ws.active = 7

	if got := c.tabWindow(a, 20, 5); got != 5 {
		t.Fatalf("going back inside the window should not move it, first = %d", got)
	}

	a.ws.active = 2

	if got := c.tabWindow(a, 20, 5); got != 2 {
		t.Fatalf("first = %d, want 2", got)
	}
}

func TestTabWindowClampsAfterClosing(t *testing.T) {
	a := appWithTabs(20, 19)
	c := &chrome{}
	c.tabWindow(a, 20, 5)
	a.ws.tabs = a.ws.tabs[:8]
	a.ws.active = 7

	if got := c.tabWindow(a, 8, 5); got != 3 {
		t.Fatalf("first = %d, want 3", got)
	}
}

func TestScrollTabs(t *testing.T) {
	a := appWithTabs(20, 0)
	c := &chrome{}
	c.tabWindow(a, 20, 5)

	c.scrollTabs(30, 48, 15)

	if c.strip.first != 0 {
		t.Fatalf("a partial step should not scroll, first = %d", c.strip.first)
	}

	c.scrollTabs(30, 48, 15)

	if c.strip.first != 1 {
		t.Fatalf("first = %d, want 1", c.strip.first)
	}

	if got := c.tabWindow(a, 20, 5); got != 1 {
		t.Fatalf("the wheel should win over the active tab, first = %d", got)
	}

	c.scrollTabs(48*100, 48, 15)

	if c.strip.first != 15 {
		t.Fatalf("first = %d, want 15", c.strip.first)
	}

	a.ws.active = 4

	if got := c.tabWindow(a, 20, 5); got != 4 {
		t.Fatalf("picking a tab should bring it back into view, first = %d", got)
	}
}

func TestHoldWidthWhileClosing(t *testing.T) {
	a := appWithTabs(4, 0)
	c := &chrome{}
	c.strip.over = true

	if got := c.holdWidth(a, 100); got != 100 {
		t.Fatalf("width = %d, want 100", got)
	}

	a.ws.tabs[1].closing = true

	if got := c.holdWidth(a, 130); got != 100 {
		t.Fatalf("tabs should keep their width under the pointer, got %d", got)
	}

	a.ws.tabs[1].closing = false

	if got := c.holdWidth(a, 130); got != 100 {
		t.Fatalf("the width should hold until the pointer leaves, got %d", got)
	}

	c.strip.over = false

	if got := c.holdWidth(a, 130); got != 130 {
		t.Fatalf("width = %d, want 130 once the pointer leaves", got)
	}
}
