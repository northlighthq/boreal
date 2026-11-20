package hosts

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ungrouped = "Hosts"

type rowState struct {
	open widget.Clickable
	more widget.Clickable
}

type List struct {
	h      ui.Host
	search widget.Editor
	list   widget.List
	rows   map[string]*rowState
	add    widget.Clickable
	empty  widget.Clickable
	count  int
}

func NewList(h ui.Host) *List {
	l := &List{h: h, rows: map[string]*rowState{}}
	ui.SetupField(&l.search, key.HintText, false)
	return l
}

func (l *List) Title() string { return "Hosts" }

func (l *List) Subtitle() string {
	switch l.count {
	case 0:
		return ""
	case 1:
		return "1 server"
	}

	return strconv.Itoa(l.count) + " servers"
}

func (l *List) Actions() []ui.Action {
	if l.count == 0 {
		return nil
	}

	return []ui.Action{{Icon: icons.Add, Click: &l.add, Do: func() { l.h.Push(NewEditor(l.h, vault.Host{})) }}}
}

func (l *List) Layout(gtx C) D {
	k := l.h.Kit()
	all := l.h.Data().Hosts
	l.count = len(all)

	if l.add.Clicked(gtx) || l.empty.Clicked(gtx) {
		l.h.Push(NewEditor(l.h, vault.Host{}))
	}

	for _, h := range all {
		r := l.state(h.ID)

		if r.open.Clicked(gtx) {
			l.h.Connect(h)
		}

		if r.more.Clicked(gtx) {
			l.h.ShowSheet(l.menu(h))
		}
	}

	if len(all) == 0 {
		return ui.Empty(gtx, k, nil, "No hosts yet",
			"Add a server once and connect to it with a tap. Hosts, keys and passwords stay encrypted on this device.",
			func(gtx C) D { return ui.Button(gtx, k, &l.empty, ui.Primary, icons.Add, "Add host", true) })
	}

	query := strings.ToLower(strings.TrimSpace(l.search.Text()))
	groups := group(filter(all, query))
	children := []ui.W{func(gtx C) D { return ui.Search(gtx, k, &l.search, "Search hosts") }}

	if len(groups) == 0 {
		children = append(children, func(gtx C) D {
			return ui.Caption(gtx, k, fmt.Sprintf("Nothing matches “%s”.", l.search.Text()))
		})
	}

	for _, g := range groups {
		rows := make([]ui.W, len(g.hosts))

		for i, h := range g.hosts {
			rows[i] = func(gtx C) D { return l.row(gtx, h) }
		}

		caption := g.name

		if len(groups) == 1 && g.name == ungrouped {
			caption = ""
		}

		meta := ""

		if caption != "" {
			meta = strconv.Itoa(len(g.hosts))
		}

		children = append(children, func(gtx C) D { return ui.SectionMeta(gtx, k, caption, meta, rows...) })
	}

	return ui.Page(gtx, &l.list, children...)
}

func (l *List) row(gtx C, h vault.Host) D {
	k := l.h.Kit()
	r := l.state(h.ID)
	ic, tint := Icon(l.h.Settings(), h)

	return ui.Row(gtx, k, &r.open, ui.RowOpts{
		Lead:     ui.TileFor(k, ic, tint),
		Title:    h.Title(),
		Subtitle: Target(h),
		Mono:     true,
		Value:    ui.Ago(h.LastUsed, time.Now()),
		Trailing: func(gtx C) D { return ui.IconButton(gtx, k, &r.more, icons.LucideMore, k.Th.TextFaint) },
	})
}

func (l *List) menu(h vault.Host) *ui.Sheet {
	return &ui.Sheet{
		Title:    h.Title(),
		Subtitle: Target(h),
		Items: []ui.SheetItem{
			{Icon: icons.Terminal, Label: "Connect", Do: func() { l.h.Connect(h) }},
			{Icon: icons.Edit, Label: "Edit", Do: func() { l.h.Push(NewEditor(l.h, h)) }},
			{Icon: icons.Copy, Label: "Duplicate", Do: func() {
				c := h
				c.ID, c.Label = "", h.Title()+" copy"
				l.h.Push(NewEditor(l.h, c))
			}},
			{Icon: icons.Delete, Label: "Delete", Danger: true, Do: func() { confirmDelete(l.h, h, nil) }},
		},
	}
}

func (l *List) state(id string) *rowState {
	r := l.rows[id]

	if r == nil {
		r = new(rowState)
		l.rows[id] = r
	}

	return r
}

func confirmDelete(h ui.Host, host vault.Host, done func()) {
	h.Ask(&ui.Dialog{
		Title:   "Delete " + host.Title() + "?",
		Message: "The host and its saved password are removed from this device. Keys stay in Keys.",
		Confirm: "Delete",
		Cancel:  "Cancel",
		Danger:  true,
		Reply: func(ok bool, _ string) {
			if !ok {
				return
			}

			if err := h.Vault().DeleteHost(host.ID); err != nil {
				h.Notify("Couldn't delete the host", err.Error())
				return
			}

			if done != nil {
				done()
			}
		},
	})
}

type hostGroup struct {
	name  string
	hosts []vault.Host
}

func filter(all []vault.Host, q string) []vault.Host {
	if q == "" {
		return all
	}

	var out []vault.Host

	for _, h := range all {
		hay := strings.ToLower(h.Title() + " " + h.Address + " " + h.Username + " " + h.Group + " " + strings.Join(h.Tags, " "))

		if strings.Contains(hay, q) {
			out = append(out, h)
		}
	}

	return out
}

func group(hosts []vault.Host) []hostGroup {
	by := map[string][]vault.Host{}

	for _, h := range hosts {
		name := strings.TrimSpace(h.Group)

		if name == "" {
			name = ungrouped
		}

		by[name] = append(by[name], h)
	}

	out := make([]hostGroup, 0, len(by))

	for name, list := range by {
		slices.SortFunc(list, func(a, b vault.Host) int {
			return cmp.Compare(strings.ToLower(a.Title()), strings.ToLower(b.Title()))
		})
		out = append(out, hostGroup{name: name, hosts: list})
	}

	slices.SortFunc(out, func(a, b hostGroup) int {
		if (a.name == ungrouped) != (b.name == ungrouped) {
			if a.name == ungrouped {
				return 1
			}

			return -1
		}

		return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	return out
}

func Target(h vault.Host) string {
	s := sshx.User(h) + "@" + h.Address

	if h.Port != 0 && h.Port != 22 {
		s += ":" + strconv.Itoa(h.Port)
	}

	return s
}
