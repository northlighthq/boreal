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
)

type assistants struct {
	enabled, read, write widget.Bool
	scope                kit.Segmented
	folders              map[string]*widget.Bool
	copy                 widget.Clickable
	activity             []borealmcp.Record
	loaded               time.Time
	message              string
	failed               bool
}

func (s *assistants) folder(name string) *widget.Bool {
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

func (s *assistants) sync(p vault.MCPPolicy, folders []string) {
	s.enabled.Value, s.read.Value, s.write.Value = p.Enabled, p.Read, p.Access
	s.scope.Value = hostsFolders

	if p.AllHosts {
		s.scope.Value = hostsAll
	}

	for _, f := range append([]string{vault.RootFolder}, folders...) {
		s.folder(f).Value = slices.Contains(p.Folders, f)
	}
}

func (s *assistants) update(a base.Host, gtx C) {
	v := a.Vault()
	p := v.MCP()
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

	if changed {
		next := vault.MCPPolicy{Enabled: s.enabled.Value, Read: s.read.Value, Access: s.write.Value, AllHosts: s.scope.Value == hostsAll}

		for _, f := range append([]string{vault.RootFolder}, folders...) {
			if s.folder(f).Value {
				next.Folders = append(next.Folders, f)
			}
		}

		err := v.SetMCP(next)
		s.failed, s.message = err != nil, ""

		if err != nil {
			s.message = err.Error()
		}

		p = v.MCP()
	}

	s.sync(p, folders)

	if s.copy.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: mimeText, Data: io.NopCloser(strings.NewReader(clientConfig()))})
		a.Toasts().Success("Copied", "Paste it into your assistant's MCP settings.")
	}

	if gtx.Now.Sub(s.loaded) > activityRefresh {
		s.activity, s.loaded = borealmcp.Recent(recentActivity), gtx.Now
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

func (d *Dialog) assistantsPage(a base.Host, gtx C, row func(string, layout.Widget) layout.FlexChild, left func(layout.Widget) layout.Widget) []layout.FlexChild {
	k := a.Kit()
	th := k.Th
	s := &d.ai
	v := a.Vault()

	muted := func(text string) layout.Widget {
		return func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 4}, text)
		}
	}

	toggle := func(b *widget.Bool, text string) layout.Widget {
		return left(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return k.Toggle(gtx, b) }),
				layout.Rigid(kit.Gap(12)),
				layout.Flexed(1, muted(text)),
			)
		})
	}

	section := func(title string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: 10, Bottom: 14}.Layout(gtx, func(gtx C) D { return k.SectionTitle(gtx, title) })
		})
	}

	note := func(text string, c kit.TextStyle) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx C) D { return k.Label(gtx, c, text) })
		})
	}

	children := []layout.FlexChild{
		note("Assistants like Claude can use Boreal's hosts and keys through the Model Context Protocol. It stays off until you turn it on here, and an imported file can never turn it on.", kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 4}),
		row("Allow assistants", toggle(&s.enabled, "Lets an assistant start Boreal's MCP server with “boreal mcp”.")),
	}

	if s.message != "" {
		children = append(children, note(s.message, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger, MaxLines: 3}))
	}

	if !s.enabled.Value {
		return children
	}

	if v.HasPassword() && !v.OSUnlock() {
		children = append(children, note("Your vault only opens with the master password, which an assistant can't type. Turn on unlocking with Windows in Security for MCP to work.", kit.TextStyle{Size: th.TextSizeSmall, Color: th.Warning, MaxLines: 3}))
	}

	children = append(children,
		section("What assistants may do"),
		row("Read", toggle(&s.read, "List hosts, keys and snippets, and see which keys each host accepts. Passwords and private keys are never shared.")),
		row("Change access", toggle(&s.write, "Add and remove keys in a host's authorized_keys. Boreal backs the file up first and never removes the key it signs in with.")),
		section("Hosts they can reach"),
		row("Hosts", left(func(gtx C) D { return k.Segmented(gtx, &s.scope, []string{"All hosts", "Chosen folders"}) })),
	)

	if s.scope.Value == hostsFolders {
		folders := append([]string{vault.RootFolder}, v.Snapshot().AllFolders()...)

		for _, f := range folders {
			label := strings.ReplaceAll(f, vault.FolderSep, " › ")

			if f == vault.RootFolder {
				label = "Hosts without a folder"
			}

			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: settingsLabelWidth, Bottom: 10}.Layout(gtx, func(gtx C) D { return k.Checkbox(gtx, s.folder(f), label) })
			}))
		}

		if !v.MCP().Scoped() {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: settingsLabelWidth, Bottom: 14}.Layout(gtx, muted("No folders chosen yet, so assistants can't reach any host."))
			}))
		}
	}

	children = append(children,
		section("Connect an assistant"),
		note("Add this to your assistant's MCP settings, like claude_desktop_config.json:", kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 2}),
		note(clientConfig(), kit.TextStyle{Size: th.TextSizeSmall, Mono: true, MaxLines: 12}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx C) D { return k.Button(gtx, &s.copy, kit.Secondary, icons.Copy, "Copy") })
		}),
		section("Recent activity"),
	)

	if len(s.activity) == 0 {
		children = append(children, note("Nothing yet.", kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}))
	}

	for i := len(s.activity) - 1; i >= 0; i-- {
		r := s.activity[i]
		text := r.Time.Local().Format(timeLayout) + "  " + r.Tool

		if r.Host != "" {
			text += "  " + r.Host
		}

		c := th.TextMuted

		if r.Outcome != borealmcp.OutcomeOK {
			c = th.Danger
			text += "  · " + r.Error
		}

		children = append(children, note(text, kit.TextStyle{Size: th.TextSizeSmall, Color: c, Mono: true, MaxLines: 2}))
	}

	return children
}
