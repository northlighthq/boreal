package mobile

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type tab int

const (
	tabHosts tab = iota
	tabSessions
	tabKeys
	tabSettings
	tabCount
)

const (
	rootBarHeight   = unit.Dp(84)
	pushBarHeight   = unit.Dp(56)
	bottomBarHeight = unit.Dp(68)
	pillWidth       = unit.Dp(60)
	pillHeight      = unit.Dp(32)
)

var tabItems = [tabCount]struct {
	label string
	icon  *icons.Icon
}{
	tabHosts:    {"Hosts", icons.LucideServer},
	tabSessions: {"Sessions", icons.Terminal},
	tabKeys:     {"Keys", icons.LucideKey},
	tabSettings: {"Settings", icons.LucideGear},
}

func (a *App) topBar(gtx C) D {
	if a.stack().CanPop() {
		return a.pushBar(gtx)
	}

	k := a.kit
	th := k.Th
	top := a.stack().Top()
	sub := ""

	if s, ok := top.(ui.Subtitled); ok {
		sub = s.Subtitle()
	}

	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(rootBarHeight)))

	return layout.Inset{Left: 20, Right: 8, Bottom: 10}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.End}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.SW.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if sub == "" {
								return D{}
							}

							return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: th.TextMuted, Weight: font.Medium, MaxLines: 1}, sub)
						}),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: ui.TextLarge, Weight: font.Bold, MaxLines: 1}, top.Title())
						}),
					)
				})
			}),
			layout.Rigid(a.actionButtons),
		)
	})
}

func (a *App) pushBar(gtx C) D {
	k := a.kit
	th := k.Th
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(pushBarHeight)))

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 4}.Layout(gtx, func(gtx C) D {
				return ui.IconButton(gtx, k, &a.back, icons.LucideArrowLeft, th.Text)
			})
		}),
		layout.Rigid(ui.Gap(4)),
		layout.Flexed(1, func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: ui.TextBar, Weight: font.SemiBold, MaxLines: 1}, a.stack().Top().Title())
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: 4}.Layout(gtx, a.actionButtons)
		}),
	)
}

func (a *App) actionButtons(gtx C) D {
	k := a.kit
	children := make([]layout.FlexChild, len(a.actions))

	for i, ac := range a.actions {
		if ac.Click.Clicked(gtx) && ac.Do != nil {
			ac.Do()
		}

		children[i] = layout.Rigid(func(gtx C) D { return ui.IconButton(gtx, k, ac.Click, ac.Icon, k.Th.Accent) })
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (a *App) bottomBar(gtx C) D {
	k := a.kit
	th := k.Th
	w := gtx.Constraints.Max.X
	h := gtx.Dp(bottomBarHeight)
	kit.Rect(gtx, image.Rect(0, 0, w, h), th.Panel)
	kit.Rect(gtx, image.Rect(0, 0, w, max(gtx.Dp(1)/2, 1)), th.Divider)

	children := make([]layout.FlexChild, tabCount)

	for i := range tabCount {
		children[i] = layout.Flexed(1, func(gtx C) D { return a.tabButton(gtx, i) })
	}

	gtx.Constraints = layout.Exact(image.Pt(w, h))
	return layout.Flex{}.Layout(gtx, children...)
}

func (a *App) tabButton(gtx C, t tab) D {
	k := a.kit
	th := k.Th
	item := tabItems[t]
	active := t == a.tab
	on := k.Animated(gtx, kit.Key(&a.tabBtns[t], 0), kit.B2F(active), kit.Normal)
	fg := kit.Lerp(th.TextMuted, th.Accent, on)
	badge := t == tabSessions && len(a.sessions) > 0

	return a.tabBtns[t].Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					size := image.Pt(gtx.Dp(pillWidth), gtx.Dp(pillHeight))

					if on > 0 {
						kit.RRect(gtx, image.Rectangle{Max: size}, size.Y/2, kit.Fade(kit.Fade(th.Accent, 0.16), on))
					}

					if badge {
						side := gtx.Dp(8)
						x := size.X/2 + gtx.Dp(9)
						y := gtx.Dp(5)
						kit.RRect(gtx, image.Rect(x, y, x+side, y+side), side/2, th.Accent)
					}

					gtx.Constraints = layout.Exact(size)
					return layout.Center.Layout(gtx, func(gtx C) D { return item.icon.Layout(gtx, 22, fg) })
				}),
				layout.Rigid(ui.Gap(4)),
				layout.Rigid(func(gtx C) D {
					weight := font.Medium

					if active {
						weight = font.SemiBold
					}

					c := kit.Lerp(th.TextMuted, th.Text, on)
					return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: c, Weight: weight, MaxLines: 1}, item.label)
				}),
			)
		})
	})
}
