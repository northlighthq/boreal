package snippets

import (
	"sort"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/desktop/base"
)

const pickerListHeight = 320

type HostPicker struct {
	h       base.Host
	snippet vault.Snippet
	hosts   []vault.Host
	checks  []widget.Bool
	list    layout.List
	dlg     kit.Dialog
	run     widget.Clickable
	cancel  widget.Clickable
}

func NewHostPicker(h base.Host, s vault.Snippet) *HostPicker {
	hosts := h.Vault().Snapshot().Hosts

	sort.SliceStable(hosts, func(i, j int) bool {
		if hosts[i].Group != hosts[j].Group {
			return strings.ToLower(hosts[i].Group) < strings.ToLower(hosts[j].Group)
		}

		return strings.ToLower(hosts[i].Title()) < strings.ToLower(hosts[j].Title())
	})

	p := &HostPicker{h: h, snippet: s, hosts: hosts, checks: make([]widget.Bool, len(hosts))}
	p.list.Axis = layout.Vertical
	return p
}

func (p *HostPicker) Dialog() *kit.Dialog {
	return &p.dlg
}

func (p *HostPicker) selected() []string {
	var ids []string

	for i, c := range p.checks {
		if c.Value {
			ids = append(ids, p.hosts[i].ID)
		}
	}

	return ids
}

func (p *HostPicker) Layout(gtx C) bool {
	k := p.h.Kit()
	th := k.Th
	ids := p.selected()

	if p.cancel.Clicked(gtx) {
		return true
	}

	if p.run.Clicked(gtx) && len(ids) > 0 {
		p.h.RunSnippetOn(p.snippet, ids)
		return true
	}

	label := "Run"

	if len(ids) > 1 {
		label = "Run on " + strconv.Itoa(len(ids)) + " hosts"
	}

	return k.Dialog(gtx, &p.dlg, kit.DialogOpts{
		Title:    "Run “" + p.snippet.Label + "” on…",
		Subtitle: "Each host opens in its own pane of a new tab.",
		Width:    440,
		Body: func(gtx C) D {
			if len(p.hosts) == 0 {
				return k.Label(gtx, kit.TextStyle{Color: th.TextMuted}, "There are no hosts yet.")
			}

			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(pickerListHeight))
			return p.list.Layout(gtx, len(p.hosts), func(gtx C, i int) D {
				h := p.hosts[i]
				text := h.Title()

				if h.Group != "" {
					text = h.Group + " / " + text
				}

				return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
					return k.Checkbox(gtx, &p.checks[i], text)
				})
			})
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &p.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &p.run, kit.Primary, icons.Play, label) },
		),
	})
}
