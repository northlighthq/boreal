package app

import (
	"github.com/plattyssh/platty/internal/app/core"
	"github.com/plattyssh/platty/internal/app/hosts"
	"github.com/plattyssh/platty/internal/app/keychain"
	"github.com/plattyssh/platty/internal/app/onboarding"
	"github.com/plattyssh/platty/internal/app/palette"
	"github.com/plattyssh/platty/internal/app/settings"
	"github.com/plattyssh/platty/internal/app/snippets"
	"github.com/plattyssh/platty/internal/sshx"
	"github.com/plattyssh/platty/internal/ui/icons"
	"github.com/plattyssh/platty/internal/ui/theme"
	"github.com/plattyssh/platty/internal/vault"
)

func (a *App) paletteEntries() []palette.Entry {
	var out []palette.Entry
	d := a.vault.Snapshot()

	for _, h := range d.Hosts {
		h := h
		out = append(out, palette.Entry{Section: "Hosts", Title: h.Title(), Detail: h.Target(), Icon: icons.Terminal, Run: func() { a.Connect(h) }})
	}

	for _, s := range d.Snippets {
		s := s
		out = append(out, palette.Entry{Section: "Snippets", Title: s.Label, Detail: s.Command, Icon: icons.Snippet, Run: func() { a.RunSnippet(s, true) }})
	}

	for _, pr := range a.profiles {
		pr := pr
		out = append(out, palette.Entry{Section: "Actions", Title: "New terminal: " + pr.Name, Icon: icons.Terminal, Run: func() { a.NewLocalTab(pr) }})
	}

	actions := []struct {
		title, detail string
		icon          *icons.Icon
		run           func()
	}{
		{"New host", "Ctrl+Shift+N", icons.Add, func() { a.OpenDocked(core.Right, hosts.NewEditor(a, hosts.Defaults())) }},
		{"Generate SSH key", "", icons.Key, func() { a.OpenIn(a.tools.keys, keychain.NewGenerateKey(a)) }},
		{"Import ~/.ssh/config", "", icons.Download, func() { a.ImportSSHConfig() }},
		{"New port forwarding rule", "", icons.Forward, func() { a.tools.show(a, a.tools.forwards) }},
		{"New snippet", "", icons.Snippet, func() { a.OpenIn(a.tools.snippets, snippets.NewEditor(a, vault.Snippet{})) }},
		{"Split right", "Ctrl+Shift+D", icons.SplitRight, func() { a.ws.split(a, true) }},
		{"Split down", "Ctrl+Shift+E", icons.SplitDown, func() { a.ws.split(a, false) }},
		{"Broadcast input to this tab", "Ctrl+Shift+B", icons.ArrowsOutline, a.toggleActiveBroadcast},
		{"Find in terminal", "Ctrl+Shift+F", icons.Search, func() {
			if pn := a.ws.activePane(); pn != nil {
				pn.view.OpenSearch()
			}
		}},
		{"Open SFTP", "Alt+6", icons.Folder, func() { a.tools.show(a, a.tools.sftp) }},
		{"Settings", "Ctrl+Alt+S", icons.Settings, func() { a.Open(settings.New(a)) }},
		{"Lock Boreal", "Ctrl+Shift+L", icons.Lock, a.LockVault},
		{"Show the introduction", "", icons.Info, func() { a.Open(onboarding.New(a)) }},
		{"Export data…", "", icons.Upload, a.ExportData},
		{"Import data…", "", icons.Download, a.ImportData},
		{"Export hosts as SSH config…", "", icons.File, a.ExportSSHConfig},
	}

	for _, act := range actions {
		out = append(out, palette.Entry{Section: "Actions", Title: act.title, Detail: act.detail, Icon: act.icon, Run: act.run})
	}

	for i, name := range theme.Names {
		name := name
		out = append(out, palette.Entry{Section: "Actions", Title: "Theme " + theme.Labels[i], Icon: icons.Tune, Run: func() { a.SetTheme(name) }})
	}

	return out
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, r := range s {
			if r == c {
				return true
			}
		}
	}

	return false
}

func (a *App) paletteQuick(q string) (palette.Entry, bool) {
	h, ok := sshx.ParseQuickConnect(q)

	if !ok || !containsAny(q, "@.:") {
		return palette.Entry{}, false
	}

	return palette.Entry{
		Section: "Quick connect",
		Title:   "Connect to " + h.Target(),
		Icon:    icons.Launch,
		Score:   1 << 20,
		Run:     func() { a.QuickConnect(q) },
	}, true
}
