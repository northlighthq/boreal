package terminal

import (
	"image/color"
	"strconv"
	"time"

	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/session"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type item struct {
	open widget.Clickable
	more widget.Clickable
}

type List struct {
	h     Host
	list  widget.List
	items map[*Session]*item
	hosts widget.Clickable
	count int
}

func NewList(h Host) *List {
	return &List{h: h, items: map[*Session]*item{}}
}

func (l *List) Title() string { return "Sessions" }

func (l *List) Subtitle() string {
	switch l.count {
	case 0:
		return ""
	case 1:
		return "1 open"
	}

	return strconv.Itoa(l.count) + " open"
}

func (l *List) Layout(gtx C) D {
	k := l.h.Kit()
	all := l.h.Sessions()
	l.count = len(all)

	for s, it := range l.items {
		if !contains(all, s) {
			delete(l.items, s)
			continue
		}

		if it.open.Clicked(gtx) {
			l.h.Open(s)
		}

		if it.more.Clicked(gtx) {
			l.h.ShowSheet(&ui.Sheet{
				Title:    s.Host.Title(),
				Subtitle: s.Host.Address,
				Items: []ui.SheetItem{
					{Icon: icons.Terminal, Label: "Open", Do: func() { l.h.Open(s) }},
					{Icon: icons.Refresh, Label: "Reconnect", Do: func() { s.Sess.Reconnect() }},
					{Icon: icons.LucideUnplug, Label: "Disconnect", Danger: true, Do: func() { l.h.Close(s) }},
				},
			})
		}
	}

	if len(all) == 0 {
		return ui.Empty(gtx, k, icons.Terminal, "No open sessions", "Connect to a host and its terminal stays here while you switch between servers.")
	}

	rows := make([]ui.W, len(all))

	for i, s := range all {
		it := l.items[s]

		if it == nil {
			it = new(item)
			l.items[s] = it
		}

		rows[i] = func(gtx C) D { return l.row(gtx, k, s, it) }
	}

	return ui.Page(gtx, &l.list, func(gtx C) D { return ui.Section(gtx, k, "", rows...) })
}

func (l *List) row(gtx C, k *kit.Kit, s *Session, it *item) D {
	th := k.Th
	label, c := StatusText(k, s)

	if s.State() == session.Connected {
		label = "Connected · " + since(s.Opened)
	}

	return ui.Row(gtx, k, &it.open, ui.RowOpts{
		Lead:     ui.TileFor(k, icons.Terminal, color.NRGBA{}),
		Title:    s.Host.Title(),
		Subtitle: label,
		Dot:      c,
		Trailing: func(gtx C) D { return ui.IconButton(gtx, k, &it.more, icons.LucideMore, th.TextFaint) },
	})
}

func contains(all []*Session, s *Session) bool {
	for _, x := range all {
		if x == s {
			return true
		}
	}

	return false
}

func since(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + " min"
	}

	return strconv.Itoa(int(d.Hours())) + " h"
}
