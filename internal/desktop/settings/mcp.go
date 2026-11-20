package settings

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/desktop/base"
	borealmcp "github.com/northlighthq/boreal/internal/mcp"
)

const (
	recentActivity  = 6
	activityRefresh = 2 * time.Second
	hostsAll        = 0
	hostsFolders    = 1
	clientConfigKey = "boreal"
	mimeText        = "application/text"
	timeLayout      = "Jan 2 15:04"
	confirmWidth    = 460
)

const mcpWarning = "Any AI client or program that can run Boreal as your user will be able to start its MCP server and use what you allow next: read your hosts, keys and snippets, and change which keys can sign in to your servers.\n\nOnly turn it on if you trust the clients you connect. Passwords and private keys are never shared, and you can turn it off at any time."

type mcpSettings struct {
	enabled, read, write widget.Bool
	scope                kit.Segmented
	folders              map[string]*widget.Bool
	copy                 widget.Clickable
	activity             []borealmcp.Record
	loaded               time.Time
	message              string
	confirming           bool
	confirmDlg           kit.Dialog
	confirmOK, cancel    widget.Clickable
}

func (s *mcpSettings) folder(name string) *widget.Bool {
	if s.folders == nil {
		s.folders = map[string]*widget.Bool{}
	}

	b := s.folders[name]

	if b == nil {
		b = new(widget.Bool)
		s.folders[name] = b
	}

	return b
}

func (s *mcpSettings) sync(p vault.MCPPolicy, folders []string) {
	s.enabled.Value, s.read.Value, s.write.Value = p.Enabled, p.Read, p.Access
	s.scope.Value = hostsFolders

	if p.AllHosts {
		s.scope.Value = hostsAll
	}

	for _, f := range append([]string{vault.RootFolder}, folders...) {
		s.folder(f).Value = slices.Contains(p.Folders, f)
	}
}

func (s *mcpSettings) policy(folders []string) vault.MCPPolicy {
	p := vault.MCPPolicy{Enabled: s.enabled.Value, Read: s.read.Value, Access: s.write.Value, AllHosts: s.scope.Value == hostsAll}

	for _, f := range append([]string{vault.RootFolder}, folders...) {
		if s.folder(f).Value {
			p.Folders = append(p.Folders, f)
		}
	}

	return p
}

func (s *mcpSettings) save(v *vault.Vault, p vault.MCPPolicy) {
	s.message = ""

	if err := v.SetMCP(p); err != nil {
		s.message = err.Error()
	}
}

func (s *mcpSettings) update(a base.Host, gtx C) {
	v := a.Vault()
	current := v.MCP()
	folders := v.Snapshot().AllFolders()
	changed := false

	for _, b := range []*widget.Bool{&s.enabled, &s.read, &s.write} {
		if b.Update(gtx) {
			changed = true
		}
	}

	if s.scope.Update(gtx) {
		changed = true
	}

	for _, f := range append([]string{vault.RootFolder}, folders...) {
		if s.folder(f).Update(gtx) {
			changed = true
		}
	}

	switch {
	case changed && s.enabled.Value && !current.Enabled:
		s.confirming = true
	case changed:
		s.save(v, s.policy(folders))
	}

	s.sync(v.MCP(), folders)

	if s.copy.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: mimeText, Data: io.NopCloser(strings.NewReader(clientConfig()))})
		a.Toasts().Success("Copied", "Paste it into your MCP client's settings.")
	}

	if gtx.Now.Sub(s.loaded) > activityRefresh {
		s.activity, s.loaded = borealmcp.Recent(recentActivity), gtx.Now
	}
}

func (s *mcpSettings) confirm(a base.Host, gtx C) {
	k := a.Kit()

	if s.confirmOK.Clicked(gtx) {
		p := a.Vault().MCP()
		p.Enabled = true
		s.save(a.Vault(), p)
		s.confirming = false
		return
	}

	if s.cancel.Clicked(gtx) {
		s.confirming = false
		return
	}

	closed := k.Dialog(gtx, &s.confirmDlg, kit.DialogOpts{
		Title: "Turn on MCP?",
		Width: confirmWidth,
		Body: func(gtx C) D {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Right: 12, Top: 2}.Layout(gtx, func(gtx C) D { return icons.Shield.Layout(gtx, 20, k.Th.Warning) })
				}),
				layout.Flexed(1, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Color: k.Th.TextMuted, MaxLines: 12}, mcpWarning)
				}),
			)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &s.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &s.confirmOK, kit.Danger, nil, "Turn on MCP") },
		),
	})

	if closed {
		s.confirming = false
	}
}

func clientConfig() string {
	exe, err := os.Executable()

	if err != nil {
		exe = "boreal"
	}

	cfg := map[string]any{"mcpServers": map[string]any{clientConfigKey: map[string]any{"command": exe, "args": []string{borealmcp.Command}}}}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
}

func (d *Dialog) mcpPage(a base.Host, p page) []layout.FlexChild {
	k := p.k
	th := k.Th
	s := &d.mcp
	v := a.Vault()

	main := []layout.Widget{
		p.item("Allow MCP", "Lets an MCP client start Boreal's server with “boreal mcp”. Off by default, and an imported file can never turn it on.", p.toggle(&s.enabled)),
	}

	if s.message != "" {
		main = append(main, p.note(s.message, th.Danger))
	}

	if s.enabled.Value && v.HasPassword() && !v.OSUnlock() {
		main = append(main, p.note("Your vault only opens with the master password, which an MCP client can't type. Turn on unlocking with "+vault.OSName+" in Security for MCP to work.", th.Warning))
	}

	children := []layout.FlexChild{
		p.header(settingsPages[pageMCP].title),
		p.group("Model Context Protocol", main...),
	}

	if !s.enabled.Value {
		return children
	}

	hosts := []layout.Widget{
		p.item("Reach", "Every host, or only the folders you pick.", p.segmented(&s.scope, []string{"All hosts", "Chosen folders"})),
	}

	if s.scope.Value == hostsFolders {
		folders := append([]string{vault.RootFolder}, v.Snapshot().AllFolders()...)

		hosts = append(hosts, p.block(func(gtx C) D {
			rows := make([]layout.FlexChild, 0, len(folders)+1)

			for i, f := range folders {
				label := strings.ReplaceAll(f, vault.FolderSep, " › ")

				if f == vault.RootFolder {
					label = "Hosts without a folder"
				}

				rows = append(rows, layout.Rigid(func(gtx C) D {
					in := layout.Inset{}

					if i > 0 {
						in.Top = 10
					}

					return in.Layout(gtx, func(gtx C) D { return k.Checkbox(gtx, s.folder(f), label) })
				}))
			}

			if !v.MCP().Scoped() {
				rows = append(rows, layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Warning, MaxLines: 2}, "No folders chosen yet, so MCP clients can't reach any host.")
					})
				}))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}))
	}

	activity := []layout.Widget{}

	if len(s.activity) == 0 {
		activity = append(activity, p.note("Nothing yet.", th.TextMuted))
	}

	for i := len(s.activity) - 1; i >= 0; i-- {
		r := s.activity[i]
		title := r.Tool

		if r.Host != "" {
			title += " · " + r.Host
		}

		desc, c := r.Time.Local().Format(timeLayout), th.TextMuted

		if r.Outcome != borealmcp.OutcomeOK {
			desc, c = desc+" · "+r.Error, th.Danger
		}

		activity = append(activity, p.itemColor(title, desc, c, nil))
	}

	return append(children,
		p.group("Permissions",
			p.item("Read", "List hosts, keys and snippets, and see which keys each host accepts.", p.toggle(&s.read)),
			p.item("Change access", "Add and remove keys in a host's authorized_keys. Boreal backs the file up first and never removes the key it signs in with.", p.toggle(&s.write)),
		),
		p.group("Hosts", hosts...),
		p.group("Connect a client",
			p.note("Add this to your MCP client's configuration:", th.TextMuted),
			p.block(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Mono: true, MaxLines: 12}, clientConfig())
			}),
			p.block(func(gtx C) D { return k.Button(gtx, &s.copy, kit.Secondary, icons.Copy, "Copy configuration") }),
		),
		p.group("Recent activity", activity...),
	)
}
