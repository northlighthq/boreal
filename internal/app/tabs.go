package app

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	tabHeight = unit.Dp(36)
	tabInfoW  = unit.Dp(320)
)

func (c *chrome) tabStrip(a *App, gtx C) D {
	k := a.kit
	th := a.th()
	c.handleTabMenu(a)

	if c.newTab.Clicked(gtx) {
		a.NewLocalTab(a.DefaultProfile())
	}

	if i, ok := c.tabMenu.Chosen(); ok && i < len(c.hiddenTabs) {
		a.ws.active = c.hiddenTabs[i]
		a.ws.focusActive()
	}

	tabs := a.ws.tabs
	h := gtx.Constraints.Max.Y
	plus := gtx.Dp(28) + gtx.Dp(6)
	more := gtx.Dp(48)
	avail := gtx.Constraints.Max.X - plus
	minW, maxW := gtx.Dp(tabMinWidth), gtx.Dp(tabMaxWidth)

	n := len(tabs)
	fit := n

	if n*minW > avail {
		fit = max(1, (avail-more)/minW)
	}

	w := maxW

	if fit > 0 {
		w = min(maxW, (avail-b2i(fit < n)*more)/fit)
	}

	visible := make([]int, 0, fit)

	for i := 0; i < n && len(visible) < fit; i++ {
		visible = append(visible, i)
	}

	if act := a.ws.active; fit < n && act >= fit {
		visible[len(visible)-1] = act
	}

	shown := map[int]bool{}

	for _, i := range visible {
		shown[i] = true
	}

	c.hiddenTabs = c.hiddenTabs[:0]

	for i, t := range tabs {
		if !shown[i] && !t.closing {
			c.hiddenTabs = append(c.hiddenTabs, i)
		}

		if !shown[i] && t.closing {
			t.vanished = true
		}
	}

	x := 0
	pos := map[int][2]int{}

	for _, i := range visible {
		t := tabs[i]
		v := t.appear.Animate(gtx, 1, kit.Slow)

		if t.closing {
			if v = t.vanish.Animate(gtx, 0, kit.Normal); v <= 0.001 {
				t.vanished = true
				continue
			}
		}

		pos[i] = [2]int{x, int(float32(w) * v)}
		x += pos[i][1]
	}

	c.tabPos = map[*Tab][2]int{}

	for i, p := range pos {
		c.tabPos[tabs[i]] = p
	}

	if p, ok := pos[a.ws.active]; ok {
		ax := int(k.Animated(gtx, kit.Key(&c.tabPos, 0), float32(p[0]), kit.Normal))
		aw := int(k.Animated(gtx, kit.Key(&c.tabPos, 1), float32(p[1]), kit.Normal))
		th0 := gtx.Dp(tabHeight)
		st := op.Offset(image.Pt(ax, h-th0)).Push(gtx.Ops)
		c.activeTab(a, gtx, aw, th0)
		st.Pop()
	}

	for j, i := range visible {
		p, ok := pos[i]

		if !ok || i == a.ws.active {
			continue
		}

		c.tab(a, gtx, i, p[0], p[1], false)
		next := -1

		if j+1 < len(visible) {
			next = visible[j+1]
		}

		if next != a.ws.active && !tabs[i].row.Hovered() && (next < 0 || !tabs[next].row.Hovered()) {
			lh := gtx.Dp(16)
			top := h - gtx.Dp(tabHeight) + (gtx.Dp(tabHeight)-lh)/2
			kit.Rect(gtx, image.Rect(p[0]+p[1]-1, top, p[0]+p[1], top+lh), th.Divider)
		}
	}

	if p, ok := pos[a.ws.active]; ok {
		c.tab(a, gtx, a.ws.active, p[0], p[1], true)
	}

	top := h - gtx.Dp(tabHeight) + (gtx.Dp(tabHeight)-gtx.Dp(28))/2

	if len(c.hiddenTabs) > 0 {
		if c.moreTabs.Clicked(gtx) {
			items := make([]kit.MenuItem, len(c.hiddenTabs))

			for j, i := range c.hiddenTabs {
				ic := icons.Terminal

				if tabs[i].active.sess.Kind == session.SSH {
					ic = icons.Server
				}

				items[j] = kit.MenuItem{Label: tabs[i].title(), Icon: ic}
			}

			c.tabMenu.Width = 240
			c.tabMenu.Open(items, image.Pt(x, top+gtx.Dp(32)))
		}

		st := op.Offset(image.Pt(x+gtx.Dp(2), top)).Push(gtx.Ops)
		dims := c.moreTabsButton(a, gtx, len(c.hiddenTabs))
		st.Pop()
		x += dims.Size.X + gtx.Dp(2)
	}

	k.Menu(gtx, &c.tabMenu)
	k.Menu(gtx, &c.tabCtx)

	st := op.Offset(image.Pt(x+gtx.Dp(4), top)).Push(gtx.Ops)
	k.IconButtonSized(gtx, &c.newTab, icons.PlusOutline, false, 28, 16, th.TextMuted)
	st.Pop()

	return D{Size: image.Pt(max(x+plus, gtx.Constraints.Max.X), h)}
}

func (c *chrome) moreTabsButton(a *App, gtx C, hidden int) D {
	k := a.kit
	th := a.th()
	return c.moreTabs.Layout(gtx, func(gtx C) D {
		hv := k.Hover(gtx, kit.Key(&c.moreTabs, 0), c.moreTabs.Hovered())
		size := image.Pt(gtx.Dp(44), gtx.Dp(28))
		bg := kit.Lerp(kit.Fade(th.Hover, hv*2), th.Selection, kit.B2F(c.tabMenu.IsOpen()))

		if bg.A > 0 {
			kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), bg)
		}

		gtx.Constraints = layout.Exact(size)
		layout.Center.Layout(gtx, func(gtx C) D {
			fg := kit.Lerp(th.TextMuted, th.Text, hv)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: fg}, strconv.Itoa(hidden))
				}),
				layout.Rigid(kit.Gap(2)),
				layout.Rigid(func(gtx C) D { return icons.ExpandMore.Layout(gtx, 14, fg) }),
			)
		})
		kit.Cursor(gtx, size, pointer.CursorPointer)
		return D{Size: size}
	})
}

func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}

func (c *chrome) tab(a *App, gtx C, i, x, w int, active bool) {
	k := a.kit
	th := a.th()
	t := a.ws.tabs[i]

	th0 := gtx.Dp(tabHeight)
	y := gtx.Constraints.Max.Y - th0
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	size := image.Pt(w, th0)

	hv := k.Hover(gtx, kit.Key(&t.row, 9), t.row.Hovered())

	v := t.appear.Animate(gtx, 1, kit.Slow)

	if t.closing {
		v = min(v, t.vanish.Animate(gtx, 0, kit.Normal))
	}

	if v <= 0.01 {
		return
	}

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()

	switch {
	case !active && hv > 0:
		r := image.Rect(gtx.Dp(3), gtx.Dp(4), w-gtx.Dp(3), th0-gtx.Dp(4))
		kit.RRect(gtx, r, gtx.Dp(th.RadiusSmall), kit.Fade(th.Hover, hv*2))
	}

	title := t.title()

	if n := len(t.root.panes(nil)); n > 1 {
		title += " (" + strconv.Itoa(n) + ")"
	}

	fg := kit.Lerp(kit.Lerp(th.TextMuted, th.Text, hv), th.Text, kit.B2F(active))
	fg = kit.Fade(fg, v*v)

	t.row.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(size)
		layout.Inset{Left: 12, Right: 6}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return c.tabIcon(a, gtx, t, fg) }),
				layout.Rigid(func(gtx C) D {
					if !a.settings.TabIcons && !c.tabBusy(t) && !a.waiting(t) {
						return D{}
					}

					return kit.Gap(8)(gtx)
				}),
				layout.Flexed(1, func(gtx C) D {
					gtx.Constraints.Min.Y = th0
					return layout.W.Layout(gtx, func(gtx C) D {
						if c.renaming == t {
							return c.renameField(a, gtx, t)
						}

						return k.Label(gtx, kit.TextStyle{Color: fg, MaxLines: 1}, title)
					})
				}),
				layout.Rigid(func(gtx C) D {
					sz := image.Pt(gtx.Dp(22), gtx.Dp(22))

					if t.closing || !(active || t.row.Hovered() || t.close.Hovered()) {
						return D{Size: sz}
					}

					return k.IconButtonSized(gtx, &t.close, icons.Close, false, 22, 13, th.TextMuted)
				}),
			)
		})
		kit.Cursor(gtx, size, pointer.CursorPointer)
		return D{Size: size}
	})

	c.trackTab(a, gtx, t, x, size)
}

func (c *chrome) tabIcon(a *App, gtx C, t *Tab, fg color.NRGBA) D {
	th := a.th()
	px := gtx.Dp(16)
	sz := image.Pt(px, px)
	st, _ := t.active.sess.State()
	ssh := t.active.sess.Kind == session.SSH

	if a.waiting(t) {
		pulse := 0.55 + 0.45*float32(math.Cos(float64(gtx.Now.UnixMilli()%1400)/1400*2*math.Pi))
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(50 * time.Millisecond)})
		return icons.Lock.Layout(gtx, 16, kit.Fade(th.Accent, pulse))
	}

	switch {
	case ssh && st == session.Connecting:
		spinner(gtx, px, fg, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
		return D{Size: sz}
	case ssh && st == session.Failed:
		gtx.Constraints = layout.Exact(sz)
		layout.Center.Layout(gtx, func(gtx C) D { return kit.Dot(gtx, th.Danger, 8) })
		return D{Size: sz}
	}

	if !a.settings.TabIcons {
		return D{}
	}

	if t.active.sess.Kind == session.SSH {
		return icons.Server.Layout(gtx, 16, fg)
	}

	return icons.Terminal.Layout(gtx, 16, fg)
}

func (a *App) waiting(t *Tab) bool {
	for _, p := range t.root.panes(nil) {
		if p.asker != nil && a.prompts.find(p.asker) != nil {
			return true
		}
	}

	return false
}

func (c *chrome) tabBusy(t *Tab) bool {
	st, _ := t.active.sess.State()
	return t.active.sess.Kind == session.SSH && (st == session.Connecting || st == session.Failed)
}

func (c *chrome) activeTab(a *App, gtx C, w, h int) {
	if w < gtx.Dp(24) {
		return
	}

	th := a.th()

	switch a.settings.TabStyle {
	case "pill":
		in := gtx.Dp(4)
		kit.RRect(gtx, image.Rect(gtx.Dp(2), in, w-gtx.Dp(2), h-in), (h-2*in)/2, th.Selection)
	case "underline":
		bar := gtx.Dp(2)
		pad := gtx.Dp(10)
		kit.RRect(gtx, image.Rect(pad, h-bar-gtx.Dp(2), w-pad, h-gtx.Dp(2)), bar/2, th.Accent)
	default:
		paint.FillShape(gtx.Ops, th.Editor, clip.Outline{Path: tabShape(gtx, image.Pt(w, h+1))}.Op())
	}
}

func spinner(gtx C, px int, c color.NRGBA, now time.Time) {
	s := float32(px)
	center := f32.Pt(s/2, s/2)
	r := s * 0.38
	turn := float64(now.UnixMilli()%900) / 900 * 2 * math.Pi
	start := f32.Pt(center.X+r*float32(math.Cos(turn)), center.Y+r*float32(math.Sin(turn)))

	track := clip.Ellipse{Min: image.Pt(int(center.X-r), int(center.Y-r)), Max: image.Pt(int(center.X+r), int(center.Y+r))}
	paint.FillShape(gtx.Ops, kit.Fade(c, 0.2), clip.Stroke{Path: track.Path(gtx.Ops), Width: max(s/9, 1.5)}.Op())

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(start)
	p.Arc(center.Sub(start), center.Sub(start), 1.5*math.Pi)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: max(s/9, 1.5)}.Op())
}

func tabShape(gtx C, size image.Point) clip.PathSpec {
	w, h := float32(size.X), float32(size.Y)
	r := float32(gtx.Dp(9))
	f := float32(gtx.Dp(7))
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(-f, h))
	p.QuadTo(f32.Pt(0, h), f32.Pt(0, h-f))
	p.LineTo(f32.Pt(0, r))
	p.QuadTo(f32.Pt(0, 0), f32.Pt(r, 0))
	p.LineTo(f32.Pt(w-r, 0))
	p.QuadTo(f32.Pt(w, 0), f32.Pt(w, r))
	p.LineTo(f32.Pt(w, h-f))
	p.QuadTo(f32.Pt(w, h), f32.Pt(w+f, h))
	p.Close()
	return p.End()
}
