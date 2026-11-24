package settings

import (
	"image"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/theme"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/desktop/base"
	"github.com/northlighthq/boreal/internal/platform/urlhandler"
)

const (
	pageAppearance = iota
	pageTerminal
	pageTriggers
	pageShortcuts
	pageSSH
	pageSecurity
	pageMCP
	pageData
	pageAbout
)

const (
	sidebarWidth = unit.Dp(196)
	navRowHeight = unit.Dp(32)
	agentRefresh = 3 * time.Second
)

var settingsPages = []struct {
	title string
	icon  *icons.Icon
}{
	pageAppearance: {"Appearance", icons.Tune},
	pageTerminal:   {"Terminal", icons.Terminal},
	pageTriggers:   {"Triggers", icons.LucideBell},
	pageShortcuts:  {"Keyboard shortcuts", icons.Keyboard},
	pageSSH:        {"SSH", icons.Lock},
	pageSecurity:   {"Security", icons.Shield},
	pageMCP:        {"MCP", icons.CPU},
	pageData:       {"Data", icons.Download},
	pageAbout:      {"About", icons.Info},
}

var settingsGroups = []struct {
	title string
	pages []int
}{
	{"General", []int{pageAppearance, pageTerminal, pageTriggers, pageShortcuts}},
	{"SSH and security", []int{pageSSH, pageSecurity, pageMCP}},
	{"Boreal", []int{pageData, pageAbout}},
}

func (d *Dialog) sidebar(a base.Host, gtx C) D {
	k := a.Kit()
	th := k.Th
	w := gtx.Dp(sidebarWidth)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w

	var children []layout.FlexChild

	for gi, g := range settingsGroups {
		children = append(children, layout.Rigid(func(gtx C) D {
			top := unit.Dp(18)

			if gi == 0 {
				top = 0
			}

			return layout.Inset{Top: top, Left: 10, Bottom: 6}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint, Weight: font.Medium}, g.title)
			})
		}))

		for _, i := range g.pages {
			children = append(children, layout.Rigid(func(gtx C) D { return d.navItem(a, gtx, i, w) }))
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Dialog) navItem(a base.Host, gtx C, i, w int) D {
	k := a.Kit()
	th := k.Th
	pg := settingsPages[i]
	on := d.page == i

	return d.pages[i].Layout(gtx, func(gtx C) D {
		size := image.Pt(w, gtx.Dp(navRowHeight))

		switch {
		case on:
			kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), th.Selection)
		case d.pages[i].Hovered():
			kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), th.Hover)
		}

		gtx.Constraints = layout.Exact(size)
		layout.Inset{Left: 10}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					c := th.TextFaint

					if on {
						c = th.Accent
					}

					return pg.icon.Layout(gtx, 16, c)
				}),
				layout.Rigid(kit.Gap(10)),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.Y = size.Y
					return layout.W.Layout(gtx, func(gtx C) D {
						c := th.TextMuted

						if on {
							c = th.Text
						}

						return k.Label(gtx, kit.TextStyle{Color: c}, pg.title)
					})
				}),
			)
		})
		kit.Cursor(gtx, size, pointer.CursorPointer)
		return D{Size: size}
	})
}

func (d *Dialog) content(a base.Host, gtx C) D {
	p := page{k: a.Kit()}
	var children []layout.FlexChild

	switch d.page {
	case pageAppearance:
		children = d.appearancePage(a, p)
	case pageTerminal:
		children = d.terminalPage(a, p)
	case pageTriggers:
		children = append([]layout.FlexChild{p.header(settingsPages[pageTriggers].title)}, d.triggersPage(a, gtx)...)
	case pageShortcuts:
		children = d.shortcutsPage(p)
	case pageSSH:
		children = d.sshPage(p, gtx.Now)
	case pageSecurity:
		children = d.securityPage(a, p)
	case pageMCP:
		children = d.mcpPage(a, p)
	case pageData:
		children = d.dataPage(p)
	case pageAbout:
		children = d.aboutPage(a, gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Dialog) appearancePage(a base.Host, p page) []layout.FlexChild {
	hosts := []layout.Widget{
		p.item("Icons", "Show each host's operating system, or a terminal icon for all of them.", p.segmented(&d.hostIcon, []string{"Operating system", "Terminal"})),
	}

	if hostIconModes[d.hostIcon.Value] == config.HostIconsOS {
		hosts = append(hosts, p.item("Icon colors", "", p.segmented(&d.hostTint, []string{"Color", "Monochrome"})))
	}

	hosts = append(hosts, p.item("Hide IP addresses", "Blur addresses on screen, for screenshots and streaming.", p.toggle(&d.hideIPs)))

	return []layout.FlexChild{
		p.header(settingsPages[pageAppearance].title),
		p.group("Theme",
			p.item("Theme", "Colors for the whole app.", p.choose(&d.theme, theme.Labels)),
			p.item("Accent color", "Buttons, switches and the active item.", func(gtx C) D { return d.swatches(a, gtx) }),
		),
		p.group("Layout",
			p.item("Density", "How much room rows and panels take.", p.segmented(&d.density, []string{"Compact", "Normal", "Comfortable"})),
			p.item("Corners", "", p.segmented(&d.corners, []string{"Sharp", "Normal", "Round"})),
		),
		p.group("Tabs",
			p.item("Style", "", p.segmented(&d.tabStyle, []string{"Classic", "Pill", "Underline"})),
			p.item("Icons", "Show the host or shell icon in each tab.", p.toggle(&d.tabIcons)),
			p.item("Activity", "Brighten a background tab when its terminal prints something new.", p.toggle(&d.tabActivity)),
		),
		p.group("Hosts", hosts...),
	}
}

func (d *Dialog) terminalPage(a base.Host, p page) []layout.FlexChild {
	names := make([]string, len(a.Profiles()))

	for i, pr := range a.Profiles() {
		names[i] = pr.Name
	}

	return []layout.FlexChild{
		p.header(settingsPages[pageTerminal].title),
		p.group("Shell",
			p.item("Default shell", "Opens with Ctrl+Shift+T and the + button.", p.choose(&d.shell, names)),
			p.item("Reopen tabs on start", "Bring back the tabs and panes from last time.", p.toggle(&d.restore)),
		),
		p.group("Text",
			p.item("Font", "", p.choose(&d.termFont, fonts.TerminalFonts)),
			p.item("Size", "Points, from 8 to 40.", p.field(&d.fontSize, "13", fieldWidth)),
			p.item("Line height", "From 1 to 2.", p.field(&d.lineHeight, "1.2", fieldWidth)),
			p.item("Ligatures", "Join character pairs like -> and != in fonts that support it.", p.toggle(&d.ligs)),
			p.item("Colors", "The theme's palette, or a classic terminal scheme.", p.choose(&d.scheme, theme.Schemes())),
		),
		p.group("Cursor",
			p.item("Style", "", p.segmented(&d.cur, []string{"Block", "Bar", "Underline"})),
			p.item("Blink", "", p.toggle(&d.blink)),
		),
		p.group("Output",
			p.item("Highlight", "Color IPs, URLs, paths, dates and errors.", p.toggle(&d.hl)),
			p.item("Scrollback", "Lines kept above the screen.", p.field(&d.scrollback, "10000", fieldWidth)),
		),
		p.group("Clipboard",
			p.item("Copy on select", "Copy text as soon as you select it.", p.toggle(&d.copySel)),
			p.item("Programs can copy", "When a program in the terminal asks to set the clipboard.", p.segmented(&d.clip, []string{"Ask", "Allow", "Block"})),
		),
		p.group("Broadcast",
			p.item("Leave out", "Hosts with these tags start excluded when you type into every pane. Click a pane's label to include it anyway.", p.field(&d.skipTags, "prod, production", wideField)),
		),
	}
}

func (d *Dialog) agentStatus(now time.Time) string {
	if now.Sub(d.agentAt) < agentRefresh && d.agent != "" {
		return d.agent
	}

	d.agent, d.agentAt = "Not detected. Start the “OpenSSH Authentication Agent” service, Pageant, 1Password or Bitwarden.", now

	if path, ok := sshx.AgentPath(""); ok {
		d.agent = "Found " + sshx.AgentName(path) + "."
	}

	return d.agent
}

func (d *Dialog) sshPage(p page, now time.Time) []layout.FlexChild {
	agent := d.agentStatus(now)

	children := []layout.FlexChild{
		p.header(settingsPages[pageSSH].title),
		p.group("Connections",
			p.item("Keep-alive", "Seconds between keep-alive messages, so idle sessions stay open. 0 turns them off.", p.field(&d.keepAlive, "30", fieldWidth)),
			p.item("Confirm on quit", "Ask before closing Boreal while SSH sessions or tunnels are open.", p.toggle(&d.confirm)),
			p.item("SSH agent", agent, nil),
		),
		p.group("Known hosts",
			p.item("Read known_hosts", "Trust servers already in ~/.ssh/known_hosts and respect its @revoked lines.", p.toggle(&d.sec.readKnown)),
			p.item("Write known_hosts", "Add the server keys you accept to ~/.ssh/known_hosts too.", p.toggle(&d.sec.writeKnown)),
		),
	}

	if urlhandler.Supported {
		children = append(children, p.group("Links",
			p.item("Open ssh:// links", "ssh:// and sftp:// links open in Boreal. "+urlhandler.Hint, p.toggle(&d.links)),
		))
	}

	return children
}

func (d *Dialog) shortcutsPage(p page) []layout.FlexChild {
	k := p.k
	th := k.Th
	rows := make([]layout.Widget, len(keymap))

	for i, km := range keymap {
		rows[i] = p.item(km[0], "", func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Mono: true, Size: th.TextSizeSmall}, km[1])
		})
	}

	return []layout.FlexChild{p.header(settingsPages[pageShortcuts].title), p.group("", rows...)}
}
