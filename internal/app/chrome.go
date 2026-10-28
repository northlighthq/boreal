package app

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/settings"
	"github.com/northlighthq/boreal/internal/platform/titlebar"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/metrics"
	"github.com/northlighthq/boreal/internal/ui/mark"
)

const (
	sidebarWidth      = unit.Dp(260)
	sidebarFormWidth  = unit.Dp(330)
	sidebarTopHeight  = unit.Dp(60)
	macLogoHeight     = unit.Dp(40)
	headerHeight      = unit.Dp(52)
	cardMargin        = unit.Dp(8)
	windowButtonWidth = unit.Dp(38)
	navRowHeight      = unit.Dp(32)
	activityWidth     = unit.Dp(52)
)

var closeHover = color.NRGBA{R: 0xc4, G: 0x2b, B: 0x1c, A: 0xff}

type chrome struct {
	sideToggle widget.Clickable
	home       kit.Row
	newTerm    kit.Row
	profiles   widget.Clickable
	settings   kit.Row
	upgrade    kit.Row

	search     widget.Clickable
	newTab     widget.Clickable
	moreTabs   widget.Clickable
	metrics    metrics.Bar
	tabMenu    kit.Menu
	hiddenTabs []int
	tabCtx     kit.Menu
	ctxTab     *Tab
	renaming   *Tab
	rename     kit.InlineEdit
	tabPos     map[*Tab][2]int
	splitR     widget.Clickable
	splitD     widget.Clickable
	toolPanel  widget.Clickable
	minimize   widget.Clickable
	maximize   widget.Clickable
	close      widget.Clickable
}

func (c *chrome) shell(a *App, gtx C) D {
	th := a.th()
	kit.Fill(gtx, th.Window)

	left := 0
	size := gtx.Constraints.Max
	dims := layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			d := c.activityBar(a, gtx)
			left += d.Size.X
			return d
		}),
		layout.Rigid(func(gtx C) D {
			d := c.sidebar(a, gtx)
			left += d.Size.X
			return d
		}),
		layout.Flexed(1, func(gtx C) D { return c.main(a, gtx) }),
	)
	a.palette.Area = image.Rect(left, gtx.Dp(headerHeight), size.X, size.Y)
	return dims
}

type activityItem struct {
	index int
	icon  *icons.Icon
}

func (t *tools) activity() []activityItem {
	return []activityItem{
		{t.indexOf(t.hosts), icons.LucideMonitor},
		{t.indexOf(t.sftp), icons.LucideFolder},
		{t.indexOf(t.snippets), icons.LucideCode},
		{t.indexOf(t.keys), icons.LucideKey},
		{t.indexOf(t.forwards), icons.LucideCable},
		{t.indexOf(t.known), icons.LucideFingerprint},
	}
}

func (c *chrome) activityBar(a *App, gtx C) D {
	c.handleNav(a, gtx)

	height := gtx.Constraints.Max.Y
	width := gtx.Dp(activityWidth)
	size := image.Pt(width, height)
	gtx.Constraints = layout.Exact(size)

	top := gtx.Dp(headerHeight)

	if a.trafficLights() {
		top += gtx.Dp(macLogoHeight)
	}

	moveArea(gtx, image.Pt(width, top))

	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(width, top))
			box := gtx.Dp(36)
			defer op.Offset(image.Pt((width-box)/2, top-gtx.Dp(tabHeight)/2-box/2)).Push(gtx.Ops).Pop()
			c.homeButton(a, gtx)
			return D{Size: image.Pt(width, top)}
		}),
		layout.Rigid(func(gtx C) D {
			var items []layout.FlexChild

			for _, it := range a.tools.activity() {
				i := it.index
				tw := a.tools.list[i]
				active := a.tools.open[core.Left] == i && a.docked[core.Left] == nil
				items = append(items, layout.Rigid(func(gtx C) D {
					return c.activityButton(a, gtx, &a.tools.rows[i], it.icon, tw.Title(), active, false)
				}))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		}),
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(0, gtx.Constraints.Min.Y)} }),
		layout.Rigid(func(gtx C) D {
			label := upgradeLabel(a.Update())

			if label == "" {
				return D{}
			}

			return c.activityButton(a, gtx, &c.upgrade, icons.Download, label, false, true)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return c.activityButton(a, gtx, &c.settings, icons.LucideGear, "Settings", false, false)
			})
		}),
	)

	return D{Size: size}
}

func (c *chrome) homeButton(a *App, gtx C) D {
	k := a.kit

	if ev := c.home.Update(gtx); ev.Click {
		a.ws.active = -1
	}

	box := gtx.Dp(36)
	size := image.Pt(box, box)
	g := gtx
	g.Constraints = layout.Exact(size)
	k.Row(g, &c.home, kit.RowOpts{Height: 36, Plain: true}, func(gtx C) D { return D{} })

	px := gtx.Dp(20)
	off := op.Offset(image.Pt((box-px)/2, (box-px)/2)).Push(gtx.Ops)
	mark.Layout(gtx, 20)
	off.Pop()

	if c.home.Hovered() {
		c.tooltip(a, gtx, size, "Home")
	}

	return D{Size: size}
}

func (c *chrome) activityButton(a *App, gtx C, r *kit.Row, ic *icons.Icon, label string, active, lit bool) D {
	k := a.kit
	th := a.th()
	width := gtx.Dp(activityWidth)
	height := gtx.Dp(44)
	size := image.Pt(width, height)

	hv := k.Hover(gtx, kit.Key(r, 9), r.Hovered())
	on := k.Animated(gtx, kit.Key(r, 10), kit.B2F(active), kit.Fast)
	fg := kit.Lerp(kit.Lerp(th.TextFaint, th.Text, hv), th.Text, on)

	if lit {
		fg = th.Accent
	}

	g := gtx
	g.Constraints = layout.Exact(size)
	k.Row(g, r, kit.RowOpts{Height: 44, Plain: true}, func(gtx C) D { return D{} })

	if bg := kit.Lerp(kit.Fade(th.Hover, hv), th.Selection, on); bg.A > 0 {
		box := gtx.Dp(36)
		x, y := (width-box)/2, (height-box)/2
		kit.RRect(gtx, image.Rect(x, y, x+box, y+box), gtx.Dp(th.Radius), bg)
	}

	px := gtx.Dp(20)
	off := op.Offset(image.Pt((width-px)/2, (height-px)/2)).Push(gtx.Ops)
	ic.Layout(gtx, 20, fg)
	off.Pop()

	if r.Hovered() {
		c.tooltip(a, gtx, size, label)
	}

	return D{Size: size}
}

func (c *chrome) tooltip(a *App, gtx C, anchor image.Point, label string) {
	k := a.kit
	th := a.th()
	rec := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(260), gtx.Dp(40))}
	macro := op.Record(g.Ops)
	text := layout.Inset{Left: 9, Right: 9, Top: 5, Bottom: 5}.Layout(g, func(gtx C) D {
		return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Text}, label)
	})
	body := macro.Stop()
	box := image.Rectangle{Max: text.Size}
	off := op.Offset(image.Pt(anchor.X+gtx.Dp(2), (anchor.Y-text.Size.Y)/2)).Push(gtx.Ops)
	kit.RRect(gtx, box, gtx.Dp(th.RadiusSmall), th.Popup)
	kit.Border(gtx, box, gtx.Dp(th.RadiusSmall), 1, th.Border)
	body.Add(gtx.Ops)
	off.Pop()
	op.Defer(gtx.Ops, rec.Stop())
}

func (c *chrome) sidebar(a *App, gtx C) D {
	k := a.kit

	a.ws.update(a, gtx)

	if c.sideToggle.Clicked(gtx) {
		a.tools.toggleSide(a, core.Left)
	}

	full := gtx.Dp(a.tools.width[core.Left])

	if a.docked[core.Left] != nil {
		full = gtx.Dp(sidebarFormWidth)
	}

	if i := a.tools.open[core.Left]; i >= 0 {
		a.tools.shown[core.Left] = i
	}

	target := float32(0)

	if a.tools.open[core.Left] >= 0 {
		target = float32(full)
	}

	width := int(k.Animated(gtx, kit.Key(&c.sideToggle, 0), target, kit.Slow))

	if target > 0 && width >= gtx.Dp(minPanelWidth) {
		full = width
	}

	if width <= 0 || a.tools.shown[core.Left] < 0 {
		return D{}
	}

	height := gtx.Constraints.Max.Y
	defer clip.Rect{Max: image.Pt(width, height)}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(width-full, 0)).Push(gtx.Ops).Pop()

	if p := float32(width) / float32(full); p < 1 {
		defer paint.PushOpacity(gtx.Ops, p*p).Pop()
	}

	gtx.Constraints = layout.Exact(image.Pt(full, height))
	tw := a.tools.list[a.tools.shown[core.Left]]

	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return c.sidebarTop(a, gtx) }),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Right: cardMargin, Bottom: cardMargin}.Layout(gtx, func(gtx C) D {
				return c.island(a, gtx, func(gtx C) D {
					if a.docked[core.Left] != nil {
						return layout.Inset{Left: 8, Right: 8, Top: 8}.Layout(gtx, func(gtx C) D {
							a.layoutDocked(gtx, core.Left)
							return D{Size: gtx.Constraints.Max}
						})
					}

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: 10, Right: 10, Top: 10}.Layout(gtx, func(gtx C) D { return c.searchBar(a, gtx) })
						}),
						layout.Flexed(1, func(gtx C) D {
							if tw == core.Tool(a.tools.hosts) {
								return a.tools.hosts.Sidebar(gtx)
							}

							return c.toolView(a, gtx, tw)
						}),
					)
				})
			})
		}),
	)

	if a.docked[core.Left] == nil {
		a.tools.resize(a, gtx, core.Left)
		grab := gtx.Dp(resizeGrab)
		edge := full - 1
		area := clip.Rect(image.Rect(edge-grab/2, 0, edge+grab/2, height)).Push(gtx.Ops)
		a.tools.drag[core.Left].Add(gtx.Ops)
		pointer.CursorColResize.Add(gtx.Ops)
		area.Pop()
	}

	return D{Size: image.Pt(width, height)}
}

func (c *chrome) toolView(a *App, gtx C, tw core.Tool) D {
	k := a.kit
	th := a.th()

	return layout.Inset{Left: 8, Right: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: 12, Right: 4, Top: 18, Bottom: 4}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.Y = gtx.Dp(26)
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, tw.Title())
						}),
						layout.Rigid(func(gtx C) D { return tw.Actions(gtx) }),
					)
				})
			}),
			layout.Flexed(1, func(gtx C) D { return tw.Body(gtx) }),
		)
	})
}

func (c *chrome) sidebarTop(a *App, gtx C) D {
	th := a.th()
	k := a.kit
	height := headerHeight

	if a.trafficLights() {
		height += macLogoHeight
	}

	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(height))
	gtx.Constraints = layout.Exact(size)
	moveArea(gtx, size)

	layout.Inset{Left: 6, Right: 14, Top: height - tabHeight + (tabHeight-32)/2}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				return k.IconButtonSized(gtx, &c.sideToggle, icons.SidebarLeft, false, 32, 18, th.TextFaint)
			}),
		)
	})

	return D{Size: size}
}

func (c *chrome) island(a *App, gtx C, w layout.Widget) D {
	th := a.th()
	size := gtx.Constraints.Max
	r := image.Rectangle{Max: size}
	radius := gtx.Dp(th.RadiusLarge)

	kit.RRect(gtx, r, radius, th.Editor)
	content := clip.UniformRRect(r, radius).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(size)
	w(gtx)
	content.Pop()
	kit.Border(gtx, r, radius, 1, th.Border)
	return D{Size: size}
}

func (c *chrome) handleNav(a *App, gtx C) {
	if c.search.Clicked(gtx) {
		a.palette.Show()
	}

	if ev := c.upgrade.Update(gtx); ev.Click {
		switch a.Update().State {
		case core.UpdateAvailable, core.UpdateFailed:
			a.DownloadUpdate()
		case core.UpdateReady:
			a.RestartToUpdate()
		}
	}

	if ev := c.settings.Update(gtx); ev.Click {
		a.Open(settings.New(a))
	}

	for i := range a.tools.list {
		if ev := a.tools.rows[i].Update(gtx); ev.Click {
			a.docked[core.Left] = nil
			a.tools.toggleIndex(a, i)
		}
	}
}

func (c *chrome) navRow(a *App, gtx C, r *kit.Row, ic *icons.Icon, label string, active bool, trailing layout.Widget) D {
	return c.navRowStyled(a, gtx, r, ic, label, active, active, trailing)
}

func (c *chrome) navRowStyled(a *App, gtx C, r *kit.Row, ic *icons.Icon, label string, active, lit bool, trailing layout.Widget) D {
	k := a.kit
	th := a.th()
	hv := k.Hover(gtx, kit.Key(r, 9), r.Hovered())
	fg := kit.Lerp(kit.Lerp(th.TextMuted, th.Text, hv), th.Text, kit.B2F(lit))

	return k.Row(gtx, r, kit.RowOpts{Selected: active, Focused: true}, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 18, fg) }),
			layout.Rigid(kit.Gap(12)),
			layout.Flexed(1, func(gtx C) D { return k.Label(gtx, kit.TextStyle{Color: fg}, label) }),
			layout.Rigid(func(gtx C) D {
				if trailing == nil {
					return D{}
				}

				return trailing(gtx)
			}),
		)
	})
}

func (c *chrome) card(a *App, gtx C) D {
	th := a.th()
	size := gtx.Constraints.Max
	r := image.Rectangle{Max: size}
	radius := gtx.Dp(th.RadiusLarge)

	kit.RRect(gtx, r, radius, th.Editor)
	content := clip.UniformRRect(r, radius).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(size)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return a.ws.layout(a, gtx) }),
		layout.Rigid(func(gtx C) D { return c.statusBar(a, gtx) }),
	)
	content.Pop()
	kit.Border(gtx, r, radius, 1, th.Border)
	return D{Size: size}
}

func (c *chrome) header(a *App, gtx C) D {
	th := a.th()
	k := a.kit
	c.handleHeader(a, gtx)

	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(headerHeight))
	gtx.Constraints = layout.Exact(size)
	moveArea(gtx, size)

	pane := a.ws.activePane()
	icon := func(clk *widget.Clickable, ic *icons.Icon, active bool) layout.FlexChild {
		return layout.Rigid(func(gtx C) D { return k.IconButtonSized(gtx, clk, ic, active, 32, 18, th.TextMuted) })
	}

	children := []layout.FlexChild{}

	if a.tools.open[core.Left] < 0 {
		children = append(children, layout.Rigid(a.trafficLightsGap), icon(&c.sideToggle, icons.SidebarLeft, false), layout.Rigid(kit.Gap(8)))
	}

	children = append(children,
		layout.Flexed(1, func(gtx C) D { return c.tabStrip(a, gtx) }),
		layout.Rigid(kit.Gap(8)),
	)

	if pane != nil {
		children = append(children, icon(&c.splitR, icons.SplitRight, false), icon(&c.splitD, icons.SplitDown, false))
	}

	if a.docked[core.Right] != nil {
		children = append(children, icon(&c.toolPanel, icons.SidebarRight, a.tools.open[core.Right] >= 0))
	}

	children = append(children,
		layout.Rigid(func(gtx C) D {
			if titlebar.Native {
				return D{}
			}

			size := image.Pt(gtx.Dp(17), gtx.Dp(16))
			kit.Rect(gtx, image.Rect(gtx.Dp(8), 0, gtx.Dp(9), size.Y), th.Divider)
			return D{Size: size}
		}),
		layout.Rigid(func(gtx C) D { return c.windowControls(a, gtx) }),
	)

	layout.Inset{Left: 12, Right: 8, Top: headerHeight - tabHeight}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = gtx.Dp(tabHeight)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
	return D{Size: size}
}

func (c *chrome) handleHeader(a *App, gtx C) {
	if c.minimize.Clicked(gtx) {
		a.win.Perform(system.ActionMinimize)
	}

	if c.maximize.Clicked(gtx) {
		if a.maximized {
			a.win.Perform(system.ActionUnmaximize)
		} else {
			a.win.Perform(system.ActionMaximize)
		}
	}

	if c.close.Clicked(gtx) {
		a.win.Perform(system.ActionClose)
	}

	if c.splitR.Clicked(gtx) {
		a.ws.split(a, true)
	}

	if c.splitD.Clicked(gtx) {
		a.ws.split(a, false)
	}

	if c.toolPanel.Clicked(gtx) {
		a.tools.toggleSide(a, core.Right)
	}
}

func (c *chrome) searchBar(a *App, gtx C) D {
	k := a.kit
	th := a.th()

	return c.search.Layout(gtx, func(gtx C) D {
		w := gtx.Constraints.Max.X
		h := gtx.Dp(32)
		size := image.Pt(w, h)
		r := image.Rectangle{Max: size}
		radius := gtx.Dp(th.RadiusSmall)

		hv := k.Hover(gtx, kit.Key(&c.search, 0), c.search.Hovered())
		kit.RRect(gtx, r, radius, kit.Lerp(kit.Fade(th.Hover, 1.4), th.Selection, hv))

		gtx.Constraints = layout.Exact(size)
		layout.Inset{Left: 10, Right: 10}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icons.Search.Layout(gtx, 16, th.TextMuted) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D {
					gtx.Constraints.Min.Y = h
					return layout.W.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 1}, "Search…")
					})
				}),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, "Ctrl+Shift+P")
				}),
			)
		})
		kit.Cursor(gtx, size, pointer.CursorPointer)
		return D{Size: size}
	})
}

func (c *chrome) windowButton(a *App, gtx C, clk *widget.Clickable, ic *icons.Icon, isClose bool) D {
	th := a.th()

	return clk.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Dp(windowButtonWidth), gtx.Dp(32))
		r := image.Rectangle{Max: size}
		radius := gtx.Dp(th.RadiusSmall)
		fg := th.TextMuted

		switch {
		case isClose && clk.Hovered():
			kit.RRect(gtx, r, radius, closeHover)
			fg = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		case clk.Hovered():
			kit.RRect(gtx, r, radius, th.Hover)
			fg = th.Text
		}

		gtx.Constraints = layout.Exact(size)
		layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 16, fg) })
		return D{Size: size}
	})
}

func (a *App) trafficLights() bool {
	return titlebar.Native && !a.fullscreen
}

func (a *App) trafficLightsGap(gtx C) D {
	if !a.trafficLights() {
		return D{}
	}

	return D{Size: image.Pt(gtx.Dp(titlebar.Width), 0)}
}

func (c *chrome) windowControls(a *App, gtx C) D {
	if titlebar.Native {
		return D{}
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return c.windowButton(a, gtx, &c.minimize, icons.Minimize, false) }),
		layout.Rigid(func(gtx C) D {
			ic := icons.Maximize

			if a.maximized {
				ic = icons.Restore
			}

			return c.windowButton(a, gtx, &c.maximize, ic, false)
		}),
		layout.Rigid(func(gtx C) D { return c.windowButton(a, gtx, &c.close, icons.WindowClose, true) }),
	)
}

func (c *chrome) main(a *App, gtx C) D {
	size := gtx.Constraints.Max
	hh := gtx.Dp(headerHeight)

	rec := op.Record(gtx.Ops)
	hg := gtx
	hg.Constraints = layout.Exact(image.Pt(size.X, hh))
	c.header(a, hg)
	header := rec.Stop()

	off := op.Offset(image.Pt(0, hh)).Push(gtx.Ops)
	bg := gtx
	bg.Constraints = layout.Exact(image.Pt(size.X, size.Y-hh))
	layout.Flex{}.Layout(bg,
		layout.Flexed(1, func(gtx C) D {
			in := layout.Inset{Bottom: cardMargin, Right: cardMargin}

			if a.tools.open[core.Left] < 0 {
				in.Left = cardMargin
			}

			if a.tools.open[core.Right] >= 0 {
				in.Right = 0
			}

			return in.Layout(gtx, func(gtx C) D { return c.card(a, gtx) })
		}),
		layout.Rigid(func(gtx C) D { return a.tools.panel(a, gtx, core.Right) }),
	)
	off.Pop()

	header.Add(gtx.Ops)
	return D{Size: size}
}

func upgradeLabel(info core.UpdateInfo) string {
	switch info.State {
	case core.UpdateAvailable:
		return "Update to " + info.Version
	case core.UpdateDownloading:
		return "Downloading update… " + strconv.Itoa(int(info.Progress*100)) + "%"
	case core.UpdateReady:
		return "Restart to update"
	case core.UpdateFailed:
		if info.Version != "" {
			return "Retry update"
		}
	}

	return ""
}
