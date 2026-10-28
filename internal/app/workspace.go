package app

import (
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/welcome"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/termview"
)

const (
	tabMaxWidth  = unit.Dp(200)
	tabMinWidth  = unit.Dp(96)
	dividerWidth = unit.Dp(1)
	dividerGrab  = unit.Dp(6)
	minRatio     = 0.15
)

type Pane struct {
	mirrored   bool
	cast       bool
	castLabel  widget.Clickable
	origin     origin
	stats      paneStats
	connecting connecting
	failure    failure
	sess       *session.Session
	view       *termview.View
	color      color.NRGBA
	icon       *icons.Icon
	asker      *int
}

type node struct {
	pane *Pane

	vertical bool
	ratio    float32
	a, b     *node
	drag     gesture.Drag
	dragFrom float32
	size     int
}

func (n *node) leaf() bool {
	return n.pane != nil
}

func (n *node) panes(out []*Pane) []*Pane {
	if n.leaf() {
		return append(out, n.pane)
	}

	out = n.a.panes(out)
	return n.b.panes(out)
}

func (n *node) find(p *Pane, parent *node) (*node, *node) {
	if n.leaf() {
		if n.pane == p {
			return n, parent
		}

		return nil, nil
	}

	if x, par := n.a.find(p, n); x != nil {
		return x, par
	}

	return n.b.find(p, n)
}

type Tab struct {
	broadcast bool
	name      string
	track     tabTrack
	root      *node
	active    *Pane
	row       widget.Clickable

	appear   kit.Anim
	vanish   kit.Anim
	closing  bool
	vanished bool
	close    widget.Clickable
	rects    map[*Pane]image.Rectangle
}

func (t *Tab) title() string {
	if t.name != "" {
		return t.name
	}

	if t.active == nil {
		return ""
	}

	return t.active.sess.Title()
}

type workspace struct {
	tabs   []*Tab
	active int

	profile widget.Clickable
	menu    kit.Menu
	recent  []string
	shown   *Tab
	swap    kit.Anim
}

func (w *workspace) current() *Tab {
	if w.active < 0 || w.active >= len(w.tabs) {
		return nil
	}

	return w.tabs[w.active]
}

func (w *workspace) activePane() *Pane {
	if t := w.current(); t != nil {
		return t.active
	}

	return nil
}

func (w *workspace) addTab(p *Pane) {
	t := &Tab{root: &node{pane: p}, active: p}
	t.appear.Set(0)
	w.tabs = append(w.tabs, t)
	w.active = len(w.tabs) - 1
	p.view.Focus()
}

func (w *workspace) focusActive() {
	if p := w.activePane(); p != nil {
		p.view.Focus()
	}
}

func (w *workspace) cycle(dir int) {
	n := len(w.tabs)

	for step := 1; step <= n; step++ {
		i := ((w.active+dir*step)%n + n) % n

		if !w.tabs[i].closing {
			w.active = i
			w.focusActive()
			return
		}
	}
}

func (w *workspace) split(a *App, vertical bool) {
	t := w.current()

	if t == nil {
		a.NewLocalTab(a.DefaultProfile())
		return
	}

	cur := t.active
	var p *Pane

	if cur.sess.Kind == session.SSH {
		p = a.newSSHPane(cur.sess.Target().Host)
	} else {
		p = a.newLocalPane(a.DefaultProfile())
	}

	leaf, _ := t.root.find(cur, nil)
	*leaf = node{
		vertical: vertical,
		ratio:    0.5,
		a:        &node{pane: cur},
		b:        &node{pane: p},
	}
	t.active = p
	p.view.Focus()
}

func (w *workspace) closePane(p *Pane) {
	for ti, t := range w.tabs {
		leaf, parent := t.root.find(p, nil)

		if leaf == nil {
			continue
		}

		p.sess.Close()

		if parent == nil {
			w.closeTab(ti)
			return
		}

		sibling := parent.a

		if sibling == leaf {
			sibling = parent.b
		}

		*parent = *sibling

		if t.active == p {
			t.active = parent.panes(nil)[0]
		}

		w.focusActive()
		return
	}
}

func (w *workspace) closeActivePane(a *App) {
	if p := w.activePane(); p != nil {
		w.closePane(p)
	}
}

func (w *workspace) closeTab(i int) {
	t := w.tabs[i]

	if t.closing {
		return
	}

	for _, p := range t.root.panes(nil) {
		p.sess.Close()
	}

	t.closing = true
	t.vanish.Set(1)

	if w.active == i {
		w.active = w.neighbor(i)
	}

	w.focusActive()
}

func (w *workspace) neighbor(i int) int {
	for d := 1; d < len(w.tabs); d++ {
		for _, j := range []int{i + d, i - d} {
			if j >= 0 && j < len(w.tabs) && !w.tabs[j].closing {
				return j
			}
		}
	}

	return -1
}

func (w *workspace) sweep() {
	active := w.current()
	kept := w.tabs[:0]

	for _, t := range w.tabs {
		if t.closing && t.vanished {
			continue
		}

		kept = append(kept, t)
	}

	for i := len(kept); i < len(w.tabs); i++ {
		w.tabs[i] = nil
	}

	w.tabs = kept

	w.active = -1

	for i, t := range w.tabs {
		if t == active {
			w.active = i
		}
	}
}

func (w *workspace) closeAll() {
	for _, t := range w.tabs {
		for _, p := range t.root.panes(nil) {
			p.sess.Close()
		}
	}

	w.tabs = nil
}

func (w *workspace) reap(a *App) {
	for _, t := range w.tabs {
		if t.closing {
			continue
		}

		for _, p := range t.root.panes(nil) {
			st, _ := p.sess.State()

			if p.sess.Kind == session.Local && st == session.Closed {
				w.closePane(p)
				a.Invalidate()
				return
			}
		}
	}
}

func (w *workspace) moveFocus(dx, dy int) {
	t := w.current()

	if t == nil || t.active == nil {
		return
	}

	from, ok := t.rects[t.active]

	if !ok {
		return
	}

	c := from.Min.Add(from.Max).Div(2)
	var best *Pane
	bestDist := 1 << 30

	for p, r := range t.rects {
		if p == t.active {
			continue
		}

		pc := r.Min.Add(r.Max).Div(2)
		d := pc.Sub(c)

		if (dx != 0 && d.X*dx <= 0) || (dy != 0 && d.Y*dy <= 0) {
			continue
		}

		dist := d.X*d.X + d.Y*d.Y

		if dist < bestDist {
			best, bestDist = p, dist
		}
	}

	if best != nil {
		t.active = best
		best.view.Focus()
	}
}

func (w *workspace) applyTheme(a *App) {
	for _, t := range w.tabs {
		for _, p := range t.root.panes(nil) {
			p.view.SetTheme(a.th())
		}
	}
}

func (w *workspace) applySettings(a *App) {
	s := a.termSettings()

	for _, t := range w.tabs {
		for _, p := range t.root.panes(nil) {
			p.view.SetSettings(s)
			p.view.SetFonts(a.glyphs)
		}
	}
}

func (w *workspace) layout(a *App, gtx C) D {
	th := a.th()
	kit.Fill(gtx, th.Editor)

	t := w.current()

	if t == nil {
		return a.welcome.Layout(gtx)
	}

	a.welcome.Hidden = true

	if t.rects == nil {
		t.rects = map[*Pane]image.Rectangle{}
	}

	clear(t.rects)

	if w.shown != t {
		if w.shown != nil {
			w.swap.Set(0)
		}

		w.shown = t
	}

	fade := 0.35 + 0.65*w.swap.Animate(gtx, 1, kit.Normal)
	v := t.appear.Animate(gtx, 1, kit.Slow)

	if o := fade * v; o < 1 {
		defer paint.PushOpacity(gtx.Ops, max(o, 0.02)).Pop()
	}

	if v < 1 {
		defer op.Offset(image.Pt(0, int((1-v)*float32(gtx.Dp(12))))).Push(gtx.Ops).Pop()
	}

	return w.layoutNode(a, gtx, t, t.root, image.Point{})
}

func (w *workspace) update(a *App, gtx C) {
	w.sweep()

	if i, ok := w.menu.Chosen(); ok {
		switch {
		case i < len(a.profiles):
			a.NewLocalTab(a.profiles[i])
		case i > len(a.profiles) && i-len(a.profiles)-1 < len(w.recent):
			if h, ok := a.vault.Host(w.recent[i-len(a.profiles)-1]); ok {
				a.Connect(h)
			}
		}
	}

	for _, t := range w.tabs {
		for _, p := range t.root.panes(nil) {
			if p.connecting.cancelled {
				p.connecting.cancelled = false
				w.closePane(p)
			}
		}
	}

	for _, t := range w.tabs {
		syncBroadcast(t)
	}

	for i := 0; i < len(w.tabs); i++ {
		if w.tabs[i].close.Clicked(gtx) {
			w.closeTab(i)
			continue
		}

		for {
			click, ok := w.tabs[i].row.Update(gtx)

			if !ok {
				break
			}

			if w.tabs[i].closing {
				continue
			}

			w.active = i

			if click.NumClicks >= 2 {
				a.chrome.startRename(a, w.tabs[i])
				continue
			}

			w.focusActive()
		}
	}
}

func (w *workspace) newTabMenu(a *App) []kit.MenuItem {
	items := a.profileMenu()
	w.recent = w.recent[:0]

	hosts := welcome.RecentHosts(a.vault, 6)

	if len(hosts) == 0 {
		return items
	}

	items = append(items, kit.Separator())

	for _, h := range hosts {
		items = append(items, kit.MenuItem{Label: h.Title(), Icon: icons.Hosts, Shortcut: h.Address})
		w.recent = append(w.recent, h.ID)
	}

	return items
}

func (w *workspace) layoutNode(a *App, gtx C, t *Tab, n *node, origin image.Point) D {
	size := gtx.Constraints.Max

	if n.leaf() {
		gtx.Constraints = layout.Exact(size)
		t.rects[n.pane] = image.Rectangle{Min: origin, Max: origin.Add(size)}
		dims := n.pane.view.Layout(gtx)

		a.paneOverlay(gtx, n.pane)

		if t.root != n && t.active != n.pane {
			kit.Rect(gtx, image.Rectangle{Max: size}, a.th().Dim)
		}

		if t.broadcast {
			a.broadcastMark(gtx, n.pane, size)
		}

		return dims
	}

	total := size.Y

	if n.vertical {
		total = size.X
	}

	n.size = total
	div := gtx.Dp(dividerWidth)

	w.dragDivider(gtx, n, total)

	first := int(float32(total-div) * n.ratio)
	second := total - div - first

	var firstSize, secondSize, divOff image.Point

	if n.vertical {
		firstSize = image.Pt(first, size.Y)
		secondSize = image.Pt(second, size.Y)
		divOff = image.Pt(first, 0)
	} else {
		firstSize = image.Pt(size.X, first)
		secondSize = image.Pt(size.X, second)
		divOff = image.Pt(0, first)
	}

	g1 := gtx
	g1.Constraints = layout.Exact(firstSize)
	w.layoutNode(a, g1, t, n.a, origin)

	off := divOff.Add(image.Pt(0, 0))

	if n.vertical {
		off.X += div
	} else {
		off.Y += div
	}

	st := op.Offset(off).Push(gtx.Ops)
	g2 := gtx
	g2.Constraints = layout.Exact(secondSize)
	w.layoutNode(a, g2, t, n.b, origin.Add(off))
	st.Pop()

	dr := image.Rectangle{Min: divOff, Max: divOff.Add(image.Pt(div, size.Y))}
	grab := image.Rect(divOff.X-gtx.Dp(dividerGrab)/2, 0, divOff.X+gtx.Dp(dividerGrab)/2, size.Y)
	cursor := pointer.CursorColResize

	if !n.vertical {
		dr = image.Rectangle{Min: divOff, Max: divOff.Add(image.Pt(size.X, div))}
		grab = image.Rect(0, divOff.Y-gtx.Dp(dividerGrab)/2, size.X, divOff.Y+gtx.Dp(dividerGrab)/2)
		cursor = pointer.CursorRowResize
	}

	kit.Rect(gtx, dr, a.th().Divider)

	area := clip.Rect(grab).Push(gtx.Ops)
	n.drag.Add(gtx.Ops)
	cursor.Add(gtx.Ops)
	area.Pop()

	return D{Size: size}
}

func (w *workspace) dragDivider(gtx C, n *node, total int) {
	axis := gesture.Vertical

	if n.vertical {
		axis = gesture.Horizontal
	}

	for {
		e, ok := n.drag.Update(gtx.Metric, gtx.Source, axis)

		if !ok {
			break
		}

		pos := e.Position.Y

		if n.vertical {
			pos = e.Position.X
		}

		switch e.Kind {
		case pointer.Press:
			n.dragFrom = pos - float32(total)*n.ratio
		case pointer.Drag:
			r := (pos - n.dragFrom) / float32(total)
			n.ratio = min(max(r, minRatio), 1-minRatio)
		}
	}
}

func clipRect(gtx C, size image.Point) clip.Stack {
	return clip.Rect{Max: size}.Push(gtx.Ops)
}
