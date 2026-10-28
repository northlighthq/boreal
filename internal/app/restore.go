package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/northlighthq/boreal/internal/config"
)

const sessionFile = "tabs.json"

type savedNode struct {
	Origin   *origin    `json:"origin,omitempty"`
	Vertical bool       `json:"vertical,omitempty"`
	Ratio    float32    `json:"ratio,omitempty"`
	A        *savedNode `json:"a,omitempty"`
	B        *savedNode `json:"b,omitempty"`
}

type savedTab struct {
	Name string     `json:"name,omitempty"`
	Root *savedNode `json:"root"`
}

type savedTabs struct {
	Tabs   []savedTab `json:"tabs"`
	Active int        `json:"active"`
}

func sessionPath() (string, error) {
	dir, err := config.Dir()

	if err != nil {
		return "", err
	}

	return filepath.Join(dir, sessionFile), nil
}

func saveNode(n *node) *savedNode {
	if n == nil {
		return nil
	}

	if n.leaf() {
		if n.pane.origin.empty() {
			return nil
		}

		o := n.pane.origin
		return &savedNode{Origin: &o}
	}

	a, b := saveNode(n.a), saveNode(n.b)

	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}

	return &savedNode{Vertical: n.vertical, Ratio: n.ratio, A: a, B: b}
}

func (a *App) saveTabs() {
	if path, err := sessionPath(); err == nil {
		a.saveTabsTo(path)
	}
}

func (a *App) saveTabsTo(path string) {

	if !a.settings.RestoreTabs {
		os.Remove(path)
		return
	}

	var s savedTabs

	for i, t := range a.ws.tabs {
		if t.closing {
			continue
		}

		root := saveNode(t.root)

		if root == nil {
			continue
		}

		if i == a.ws.active {
			s.Active = len(s.Tabs)
		}

		s.Tabs = append(s.Tabs, savedTab{Name: t.name, Root: root})
	}

	raw, err := json.MarshalIndent(s, "", "  ")

	if err == nil {
		config.WriteAtomic(path, raw)
	}
}

func (a *App) restoreNode(n *savedNode) *node {
	if n == nil {
		return nil
	}

	if n.Origin != nil {
		p, ok := a.paneFrom(*n.Origin)

		if !ok {
			return nil
		}

		return &node{pane: p}
	}

	x, y := a.restoreNode(n.A), a.restoreNode(n.B)

	switch {
	case x == nil:
		return y
	case y == nil:
		return x
	}

	ratio := n.Ratio

	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}

	return &node{vertical: n.Vertical, ratio: ratio, a: x, b: y}
}

func (a *App) restoreTabs() {
	if a.restored || a.vault.Locked() {
		return
	}

	a.restored = true

	if !a.settings.RestoreTabs {
		return
	}

	if path, err := sessionPath(); err == nil {
		a.restoreTabsFrom(path)
	}
}

func (a *App) restoreTabsFrom(path string) {
	raw, err := os.ReadFile(path)

	if err != nil {
		return
	}

	var s savedTabs

	if json.Unmarshal(raw, &s) != nil {
		return
	}

	for _, st := range s.Tabs {
		root := a.restoreNode(st.Root)

		if root == nil {
			continue
		}

		panes := root.panes(nil)
		t := &Tab{root: root, active: panes[0], name: st.Name}
		t.appear.Set(0)
		a.ws.tabs = append(a.ws.tabs, t)
	}

	if len(a.ws.tabs) > 0 {
		a.ws.active = min(max(s.Active, 0), len(a.ws.tabs)-1)
		a.ws.focusActive()
	}
}
