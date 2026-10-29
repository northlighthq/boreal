package app

import (
	"image"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"

	"github.com/northlighthq/boreal/internal/platform/shell"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	tabMenuRename = iota
	tabMenuDuplicate
	tabMenuSep
	tabMenuClose
	tabMenuCloseOthers
)

const tabDragThreshold = 6

type origin struct {
	Profile string      `json:"profile,omitempty"`
	Command []string    `json:"command,omitempty"`
	Title   string      `json:"title,omitempty"`
	HostID  string      `json:"hostId,omitempty"`
	Host    *vault.Host `json:"host,omitempty"`
}

func (o origin) empty() bool {
	return o.HostID == "" && o.Host == nil && len(o.Command) == 0
}

func (a *App) paneFrom(o origin) (*Pane, bool) {
	switch {
	case o.HostID != "":
		h, ok := a.vault.Host(o.HostID)

		if !ok {
			return nil, false
		}

		return a.newSSHPane(h), true
	case o.Host != nil:
		return a.newSSHPane(*o.Host), true
	case len(o.Command) > 0:
		return a.newLocalPane(shell.Profile{ID: o.Profile, Name: o.Title, Command: o.Command}), true
	}

	return nil, false
}

func (a *App) duplicateTab(t *Tab) {
	if t.active == nil {
		return
	}

	if p, ok := a.paneFrom(t.active.origin); ok {
		a.ws.addTab(p)
	}
}

type tabTrack struct {
	start    f32.Point
	down     bool
	dragging bool
}

func (c *chrome) trackTab(a *App, gtx C, t *Tab, x int, size image.Point) {
	tr := &t.track

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tr, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})

		if !ok {
			break
		}

		e, ok := ev.(pointer.Event)

		if !ok {
			continue
		}

		switch e.Kind {
		case pointer.Press:
			if e.Buttons.Contain(pointer.ButtonSecondary) {
				c.openTabMenu(a, t, image.Pt(x+int(e.Position.X), int(e.Position.Y)+gtx.Dp(30)))
				continue
			}

			tr.start, tr.down, tr.dragging = e.Position, true, false
		case pointer.Drag:
			if !tr.down {
				continue
			}

			dx := e.Position.X - tr.start.X

			if !tr.dragging && (dx > float32(gtx.Dp(tabDragThreshold)) || -dx > float32(gtx.Dp(tabDragThreshold))) {
				tr.dragging = true
			}

			if tr.dragging {
				c.reorder(a, t, float32(x)+e.Position.X)
			}
		case pointer.Release, pointer.Cancel:
			tr.down, tr.dragging = false, false
		}
	}

	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tr)
}

func (c *chrome) reorder(a *App, t *Tab, at float32) {
	from := -1

	for i, x := range a.ws.tabs {
		if x == t {
			from = i
		}
	}

	if from < 0 {
		return
	}

	for i, x := range a.ws.tabs {
		p, ok := c.tabPos[x]

		if !ok || i == from || at < float32(p[0]) || at > float32(p[0]+p[1]) {
			continue
		}

		mid := float32(p[0] + p[1]/2)

		if (i > from && at < mid) || (i < from && at > mid) {
			continue
		}

		active := a.ws.tabs[a.ws.active]
		tabs := append(a.ws.tabs[:from:from], a.ws.tabs[from+1:]...)
		tabs = append(tabs[:i], append([]*Tab{t}, tabs[i:]...)...)
		a.ws.tabs = tabs

		for j, x := range tabs {
			if x == active {
				a.ws.active = j
			}
		}

		return
	}
}

func (c *chrome) openTabMenu(a *App, t *Tab, at image.Point) {
	c.ctxTab = t
	c.tabCtx.Width = 220
	c.tabCtx.Open([]kit.MenuItem{
		tabMenuRename:      {Label: "Rename…", Icon: icons.Edit},
		tabMenuDuplicate:   {Label: "Duplicate", Icon: icons.Copy, Disabled: t.active == nil || t.active.origin.empty()},
		tabMenuSep:         kit.Separator(),
		tabMenuClose:       {Label: "Close", Icon: icons.Close},
		tabMenuCloseOthers: {Label: "Close other tabs", Disabled: len(a.ws.tabs) < 2},
	}, at)
}

func (c *chrome) handleTabMenu(a *App) {
	i, ok := c.tabCtx.Chosen()
	t := c.ctxTab

	if !ok || t == nil {
		return
	}

	switch i {
	case tabMenuRename:
		c.startRename(a, t)
	case tabMenuDuplicate:
		a.duplicateTab(t)
	case tabMenuClose:
		for j, x := range a.ws.tabs {
			if x == t {
				a.ws.closeTab(j)
				return
			}
		}
	case tabMenuCloseOthers:
		for j := len(a.ws.tabs) - 1; j >= 0; j-- {
			if a.ws.tabs[j] != t {
				a.ws.closeTab(j)
			}
		}
	}
}

func (c *chrome) startRename(a *App, t *Tab) {
	c.renaming = t
	c.rename.Start(a.kit, t.title())
}

func (c *chrome) renameField(a *App, gtx C, t *Tab) D {
	dims, res := c.rename.Layout(gtx, a.kit, "Tab name")

	switch res {
	case kit.EditCommit:
		text := strings.TrimSpace(c.rename.Text())
		t.name = ""

		if text != t.title() {
			t.name = text
		}

		c.renaming = nil
		a.ws.focusActive()
	case kit.EditCancel:
		c.renaming = nil
		a.ws.focusActive()
	}

	return dims
}
