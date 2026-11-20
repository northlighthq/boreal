package settings

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/theme"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/desktop/base"
	"github.com/northlighthq/boreal/internal/desktop/dialogs"
	"github.com/northlighthq/boreal/internal/platform/urlhandler"
)

var cursorStyles = []string{config.CursorBlock, config.CursorBar, config.CursorUnderline}

var clipboardModes = []string{config.ClipboardAsk, config.ClipboardAllow, config.ClipboardBlock}

var hostIconModes = []string{config.HostIconsOS, config.HostIconsTerminal}

var hostIconTints = []string{config.HostIconTintColor, config.HostIconTintMono}

var keymap = [][2]string{
	{"Search everywhere", "Ctrl+Shift+P"},
	{"New local terminal", "Ctrl+Shift+T"},
	{"New host", "Ctrl+Shift+N"},
	{"Close panel", "Ctrl+Shift+W"},
	{"Split right / down", "Ctrl+Shift+D / Ctrl+Shift+E"},
	{"Broadcast input to the tab", "Ctrl+Shift+B"},
	{"Move focus between panes", "Alt+Arrows"},
	{"Next / previous tab", "Ctrl+Tab / Ctrl+Shift+Tab"},
	{"Tool windows", "Alt+1 … Alt+6"},
	{"Copy / paste", "Ctrl+Shift+C / Ctrl+Shift+V"},
	{"Copy (with selection)", "Ctrl+C"},
	{"Find in terminal", "Ctrl+Shift+F"},
	{"Select all", "Ctrl+Shift+A"},
	{"Scroll history", "Shift+PgUp / Shift+PgDn"},
	{"Font size", "Ctrl+= / Ctrl+- / Ctrl+0"},
	{"Open link", "Ctrl+click"},
	{"Settings", "Ctrl+Alt+S · Ctrl+,"},
	{"Lock Boreal", "Ctrl+Shift+L"},
}

type Dialog struct {
	h       base.Host
	sec     security
	mcp     mcpSettings
	agent   string
	agentAt time.Time
	trig    triggers
	about   about
	s       config.Settings
	page    int
	dlg     kit.Dialog

	pages       []widget.Clickable
	theme       kit.Select
	cur         kit.Segmented
	clip        kit.Segmented
	shell       kit.Select
	termFont    kit.Select
	scheme      kit.Select
	density     kit.Segmented
	corners     kit.Segmented
	tabStyle    kit.Segmented
	accents     []widget.Clickable
	accent      string
	tabIcons    widget.Bool
	tabActivity widget.Bool
	hostIcon    kit.Segmented
	hostTint    kit.Segmented

	fontSize, lineHeight, scrollback, keepAlive         widget.Editor
	blink, copySel, confirm, hideIPs, restore, ligs, hl widget.Bool
	autoUpdate                                          widget.Bool
	links                                               widget.Bool
	skipTags                                            widget.Editor

	ok, apply, cancel widget.Clickable
	list              layout.List
}

func New(a base.Host) *Dialog {
	d := &Dialog{h: a, s: a.Settings(), pages: make([]widget.Clickable, len(settingsPages))}
	d.trig.load(a)
	dialogs.SingleLine(&d.fontSize, &d.lineHeight, &d.scrollback, &d.keepAlive, &d.skipTags)
	d.fontSize.Filter = "0123456789."
	d.lineHeight.Filter = "0123456789."
	d.scrollback.Filter = "0123456789"
	d.keepAlive.Filter = "0123456789"

	s := a.Settings()

	for i, n := range theme.Names {
		if n == s.Theme {
			d.theme.Value = i
		}
	}

	for i, m := range clipboardModes {
		if m == s.Clipboard {
			d.clip.Value = i
		}
	}

	for i, c := range cursorStyles {
		if c == s.CursorStyle {
			d.cur.Value = i
		}
	}

	for i, p := range a.Profiles() {
		if p.ID == s.Shell {
			d.shell.Value = i
		}
	}

	d.fontSize.SetText(strconv.FormatFloat(float64(s.FontSize), 'g', -1, 32))
	d.lineHeight.SetText(strconv.FormatFloat(float64(s.LineHeight), 'g', -1, 32))
	d.scrollback.SetText(strconv.Itoa(s.Scrollback))
	d.keepAlive.SetText(strconv.Itoa(s.KeepAlive))
	d.skipTags.SetText(s.BroadcastSkip)
	d.blink.Value = s.CursorBlink
	d.copySel.Value = s.CopyOnSelect
	d.confirm.Value = s.ConfirmClose
	d.hideIPs.Value = s.HideAddresses
	d.restore.Value = s.RestoreTabs
	d.ligs.Value = s.Ligatures
	d.hl.Value = s.Highlight
	d.tabIcons.Value = s.TabIcons
	d.tabActivity.Value = s.TabActivity
	d.accent = s.Accent
	d.accents = make([]widget.Clickable, len(theme.Accents))
	d.density.Value = index(theme.Densities, s.Density)
	d.corners.Value = index(theme.CornerStyles, s.Corners)
	d.tabStyle.Value = index(tabStyles, s.TabStyle)
	d.hostIcon.Value = index(hostIconModes, s.HostIcons)
	d.hostTint.Value = index(hostIconTints, s.HostIconTint)
	d.scheme.Value = max(0, index(theme.Schemes(), s.TermScheme))
	d.autoUpdate.Value = s.AutoUpdate
	d.links.Value = urlhandler.Registered()

	for i, name := range fonts.TerminalFonts {
		if name == s.TerminalFont {
			d.termFont.Value = i
		}
	}

	d.sec.init(s.LockAfter)
	d.sec.readKnown.Value = s.SystemKnownHosts
	d.sec.writeKnown.Value = s.WriteKnownHosts
	d.sec.lockOnSystem.Value = s.LockOnSystemLock
	d.sec.hello.Value = s.WindowsHello
	d.list.Axis = layout.Vertical
	return d
}

func (d *Dialog) collect(a base.Host) config.Settings {
	s := d.s
	s.Theme = theme.Names[d.theme.Value]
	s.CursorStyle = cursorStyles[d.cur.Value]
	s.Clipboard = clipboardModes[d.clip.Value]
	s.TerminalFont = fonts.TerminalFonts[d.termFont.Value]

	if d.shell.Value < len(a.Profiles()) {
		s.Shell = a.Profiles()[d.shell.Value].ID
	}

	if v, err := strconv.ParseFloat(d.fontSize.Text(), 32); err == nil && v >= 8 && v <= 40 {
		s.FontSize = float32(v)
	}

	if v, err := strconv.ParseFloat(d.lineHeight.Text(), 32); err == nil && v >= 1 && v <= 2 {
		s.LineHeight = float32(v)
	}

	if v, err := strconv.Atoi(d.scrollback.Text()); err == nil && v >= 0 {
		s.Scrollback = v
	}

	if v, err := strconv.Atoi(d.keepAlive.Text()); err == nil && v >= 0 {
		s.KeepAlive = v
	}

	s.CursorBlink = d.blink.Value
	s.CopyOnSelect = d.copySel.Value
	s.ConfirmClose = d.confirm.Value
	s.HideAddresses = d.hideIPs.Value
	s.RestoreTabs = d.restore.Value
	s.Ligatures = d.ligs.Value
	s.Highlight = d.hl.Value
	s.TabIcons = d.tabIcons.Value
	s.TabActivity = d.tabActivity.Value
	s.Accent = d.accent
	s.Density = theme.Densities[d.density.Value]
	s.Corners = theme.CornerStyles[d.corners.Value]
	s.TabStyle = tabStyles[d.tabStyle.Value]
	s.HostIcons = hostIconModes[d.hostIcon.Value]
	s.HostIconTint = hostIconTints[d.hostTint.Value]
	s.TermScheme = ""

	if d.scheme.Value > 0 {
		s.TermScheme = theme.Schemes()[d.scheme.Value]
	}
	s.AutoUpdate = d.autoUpdate.Value
	s.BroadcastSkip = strings.TrimSpace(d.skipTags.Text())
	s.LockAfter = lockAfterMinutes[d.sec.lockAfter.Value]
	s.SystemKnownHosts = d.sec.readKnown.Value
	s.WriteKnownHosts = d.sec.writeKnown.Value
	s.LockOnSystemLock = d.sec.lockOnSystem.Value
	s.WindowsHello = d.sec.hello.Value
	return s
}

func (d *Dialog) commit(a base.Host) {
	s := d.collect(a)
	triggersChanged := d.trig.changed()

	if s == d.s && !triggersChanged {
		return
	}

	if triggersChanged {
		list := d.trig.collect()

		if err := a.Vault().SetTriggers(list); err != nil {
			a.Toasts().Error("Couldn't save the triggers", err.Error())
			return
		}

		d.trig.saved = list
	}

	if s != d.s {
		a.UpdateSettings(s)
		d.s = s
	}

	a.Toasts().Success("Settings saved", "Your changes are applied.")
}

func (d *Dialog) Layout(gtx C) bool {
	a := d.h
	d.sec.update(a, gtx)
	d.mcp.update(a, gtx)
	k := a.Kit()

	if d.links.Update(gtx) {
		if err := urlhandler.Set(d.links.Value); err != nil {
			a.Toasts().Error("Couldn't change the link handler", err.Error())
		}

		d.links.Value = urlhandler.Registered()
	}

	if d.ok.Clicked(gtx) {
		d.commit(a)
		return true
	}

	if d.apply.Clicked(gtx) {
		d.commit(a)
	}

	if d.cancel.Clicked(gtx) {
		return true
	}

	for i := range d.pages {
		if d.pages[i].Clicked(gtx) {
			d.page = i
		}
	}

	closed := k.Dialog(gtx, &d.dlg, kit.DialogOpts{
		Title: "Settings",
		Width: 820,
		Body: func(gtx C) D {
			h := min(gtx.Constraints.Max.Y, gtx.Dp(480))
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return d.sidebar(a, gtx) }),
				layout.Rigid(kit.Gap(32)),
				layout.Flexed(1, func(gtx C) D {
					return d.list.Layout(gtx, 1, func(gtx C, _ int) D { return d.content(a, gtx) })
				}),
			)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &d.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &d.apply, kit.Secondary, nil, "Apply") },
			func(gtx C) D { return k.Button(gtx, &d.ok, kit.Primary, nil, "OK") },
		),
	})

	if d.mcp.confirming {
		d.mcp.confirm(a, gtx)
	}

	return closed
}

func vaultSummary(a base.Host) string {
	d := a.Vault().Snapshot()
	return fmt.Sprintf("%d hosts, %d keys, %d snippets.", len(d.Hosts), len(d.Keys), len(d.Snippets))
}

type (
	C = base.C
	D = base.D
)

func (d *Dialog) SetPage(i int) {
	d.page = i
}
