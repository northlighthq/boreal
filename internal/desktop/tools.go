package desktop

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/desktop/base"
	"github.com/northlighthq/boreal/internal/desktop/forwards"
	"github.com/northlighthq/boreal/internal/desktop/hosts"
	"github.com/northlighthq/boreal/internal/desktop/keychain"
	"github.com/northlighthq/boreal/internal/desktop/knownhosts"
	"github.com/northlighthq/boreal/internal/desktop/sftp"
	"github.com/northlighthq/boreal/internal/desktop/snippets"
)

const (
	panelHeaderHeight = unit.Dp(40)
	minPanelWidth     = unit.Dp(200)
	maxPanelWidth     = unit.Dp(560)
	rightPanelWidth   = unit.Dp(340)
	resizeGrab        = unit.Dp(6)
	sidebarHidden     = "none"
)

type tools struct {
	list     []base.Tool
	rows     []kit.Row
	open     [2]int
	shown    [2]int
	width    [2]unit.Dp
	hide     [2]widget.Clickable
	drag     [2]gesture.Drag
	swap     [2]kit.Anim
	swapFrom [2]int
	dragX    [2]float32

	hosts    *hosts.Panel
	keys     *keychain.Panel
	forwards *forwards.Panel
	snippets *snippets.Panel
	known    *knownhosts.Panel
	sftp     *sftp.Panel
}

func (t *tools) init(a *App) {
	t.hosts = hosts.New(a)
	t.keys = keychain.New(a)
	t.forwards = forwards.New(a)
	t.snippets = snippets.New(a)
	t.known = knownhosts.New(a)
	t.sftp = sftp.New(a)

	t.list = []base.Tool{t.hosts, t.keys, t.forwards, t.snippets, t.known, t.sftp}
	t.rows = make([]kit.Row, len(t.list))
	t.open = [2]int{-1, -1}
	t.shown = [2]int{-1, -1}
	t.width = [2]unit.Dp{unit.Dp(a.settings.SidebarWidth), rightPanelWidth}

	if a.settings.SidebarOpen != sidebarHidden {
		t.open[base.Left] = 0
	}

	t.refresh(a)
}

func (t *tools) refresh(a *App) {
	for _, w := range t.list {
		w.Refresh()
	}
}

func (t *tools) toggleIndex(a *App, i int) {
	if i < 0 || i >= len(t.list) {
		return
	}

	s := base.Left

	if t.open[s] == i {
		t.open[s] = -1
		a.ws.focusActive()
	} else {
		t.open[s] = i
	}

	t.saveOpen(a)
}

func (t *tools) show(a *App, w base.Tool) {
	for i, x := range t.list {
		if x == w {
			t.open[base.Left] = i
		}
	}

	t.saveOpen(a)
}

func (t *tools) saveOpen(a *App) {
	a.settings.SidebarOpen = "hosts"

	if t.open[base.Left] < 0 {
		a.settings.SidebarOpen = sidebarHidden
	}
}

func (t *tools) toggleSide(a *App, s base.Side) {
	if t.open[s] >= 0 {
		t.open[s] = -1
		t.saveOpen(a)
		a.ws.focusActive()
		return
	}

	if s == base.Right && a.docked[base.Right] == nil {
		return
	}

	i := t.shown[s]

	if i < 0 {
		for j, w := range t.list {
			if w.Side() == s {
				i = j
				break
			}
		}
	}

	t.open[s] = i
	t.saveOpen(a)
}

func (t *tools) panel(a *App, gtx C, s base.Side) D {
	th := a.th()
	k := a.kit

	if t.hide[s].Clicked(gtx) {
		a.docked[s] = nil
		t.open[s] = -1
		t.saveOpen(a)
		a.ws.focusActive()
	}

	if s == base.Right && a.docked[base.Right] == nil {
		t.open[s] = -1
		return D{}
	}

	if i := t.open[s]; i >= 0 {
		t.shown[s] = i
	}

	full := gtx.Dp(t.width[s])
	target := float32(0)

	if t.open[s] >= 0 {
		target = float32(full)
	}

	width := int(k.Animated(gtx, kit.Key(&t.width[s], 0), target, kit.Slow))

	if width <= 0 || t.shown[s] < 0 {
		return D{}
	}

	tw := t.list[t.shown[s]]
	height := gtx.Constraints.Max.Y

	if t.swapFrom[s] != t.shown[s]+1 {
		t.swapFrom[s] = t.shown[s] + 1
		t.swap[s].Set(0)
	}

	fade := t.swap[s].Animate(gtx, 1, kit.Normal)
	outer := image.Pt(width, height)

	defer clip.Rect{Max: outer}.Push(gtx.Ops).Pop()

	if p := float32(width) / float32(full); p < 1 {
		defer paint.PushOpacity(gtx.Ops, p*p).Pop()
	}
	panel := image.Rect(0, 0, full, height)

	header := func(gtx C) D {
		return layout.Inset{Left: 12, Right: 4, Top: 6, Bottom: 4}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(26)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, tw.Title())
				}),
				layout.Rigid(func(gtx C) D { return tw.Actions(gtx) }),
				layout.Rigid(func(gtx C) D {
					return k.IconButtonSized(gtx, &t.hide[s], icons.Close, false, 26, 16, th.TextMuted)
				}),
			)
		})
	}

	g := gtx
	g.Constraints = layout.Exact(panel.Size())
	layout.Flex{Axis: layout.Vertical}.Layout(g,
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Left: 0, Right: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
				if s == base.Right && a.docked[base.Right] != nil {
					a.layoutDocked(gtx, base.Right)
					return D{Size: gtx.Constraints.Max}
				}

				defer paint.PushOpacity(gtx.Ops, fade).Pop()
				defer op.Offset(image.Pt(0, int((1-fade)*float32(gtx.Dp(8))))).Push(gtx.Ops).Pop()
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(header),
					layout.Flexed(1, func(gtx C) D { return tw.Body(gtx) }),
				)
			})
		}),
	)

	edge := full - 1

	if s == base.Right {
		edge = 0
	}

	t.resize(a, gtx, s)
	grab := gtx.Dp(resizeGrab)
	area := clip.Rect(image.Rect(edge-grab/2, 0, edge+grab/2, height)).Push(gtx.Ops)
	t.drag[s].Add(gtx.Ops)
	pointer.CursorColResize.Add(gtx.Ops)
	area.Pop()

	return D{Size: outer}
}

func (t *tools) resize(a *App, gtx C, s base.Side) {
	for {
		e, ok := t.drag[s].Update(gtx.Metric, gtx.Source, gesture.Horizontal)

		if !ok {
			break
		}

		switch e.Kind {
		case pointer.Press:
			t.dragX[s] = e.Position.X
		case pointer.Drag:
			delta := (e.Position.X - t.dragX[s]) / gtx.Metric.PxPerDp

			if s == base.Right {
				delta = -delta
			} else {
				t.dragX[s] = e.Position.X
			}

			w := t.width[s] + unit.Dp(delta)
			t.width[s] = min(max(w, minPanelWidth), maxPanelWidth)

			if s == base.Left {
				a.settings.SidebarWidth = float32(t.width[s])
			}
		}
	}
}

func (t *tools) indexOf(w base.Tool) int {
	for i, x := range t.list {
		if x == w {
			return i
		}
	}

	return -1
}

func (a *App) OpenDocked(s base.Side, m base.Modal) {
	d, ok := m.(base.Docker)

	if !ok {
		a.Open(m)
		return
	}

	a.palette.Close()
	d.Dialog().Docked = true
	a.docked[s] = m

	if a.tools.open[s] >= 0 {
		return
	}

	if s == base.Left {
		a.tools.open[base.Left] = 0
		a.tools.saveOpen(a)
		return
	}

	a.tools.toggleSide(a, base.Right)
}

func (a *App) OpenIn(w base.Tool, m base.Modal) {
	a.tools.show(a, w)
	a.OpenDocked(base.Right, m)
}

func (a *App) layoutDocked(gtx C, s base.Side) bool {
	m := a.docked[s]

	if m == nil {
		return false
	}

	if m.Layout(gtx) {
		a.docked[s] = nil
		a.ws.focusActive()
	}

	return true
}
