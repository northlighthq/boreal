package welcome

import (
	"image"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/hosts"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mark"
	"github.com/northlighthq/boreal/internal/vault"
)

type Screen struct {
	h      core.Host
	Hidden bool

	newTerm  kit.Row
	newHost  kit.Row
	import_  kit.Row
	commands kit.Row
	recent   map[string]*kit.Row

	hosts    []vault.Host
	hostsRev uint64
	hostsOK  bool
}

func (w *Screen) recentHosts(v *vault.Vault) []vault.Host {
	if rev := v.Revision(); !w.hostsOK || rev != w.hostsRev {
		w.hosts, w.hostsRev, w.hostsOK = RecentHosts(v, 3), rev, true
	}

	return w.hosts
}

func RecentHosts(v *vault.Vault, n int) []vault.Host {
	hosts := v.Snapshot().Hosts
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].LastUsed.After(hosts[j].LastUsed) })

	var out []vault.Host

	for _, h := range hosts {
		if h.LastUsed.IsZero() && len(out) >= 2 {
			continue
		}

		out = append(out, h)

		if len(out) == n {
			break
		}
	}

	return out
}

func (w *Screen) Layout(gtx C) D {
	a := w.h

	if ev := w.newTerm.Update(gtx); ev.Click {
		a.NewLocalTab(a.DefaultProfile())
	}

	if ev := w.newHost.Update(gtx); ev.Click {
		a.OpenDocked(core.Right, hosts.NewEditor(a, hosts.Defaults()))
	}

	if ev := w.import_.Update(gtx); ev.Click {
		a.ImportSSHConfig()
	}

	if ev := w.commands.Update(gtx); ev.Click {
		a.ShowPalette()
	}

	k := a.Kit()
	th := k.Th
	recent := w.recentHosts(a.Vault())

	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X-gtx.Dp(48), gtx.Dp(360))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X

		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D { return mark.Paint(gtx, 30, kit.Fade(th.TextFaint, 0.55)) })
			}),
			layout.Rigid(kit.Gap(40)),
			layout.Rigid(func(gtx C) D { return w.action(a, gtx, &w.newTerm, icons.Terminal, "New terminal", "") }),
			layout.Rigid(func(gtx C) D { return w.action(a, gtx, &w.newHost, icons.PlusOutline, "New host", "") }),
			layout.Rigid(func(gtx C) D { return w.action(a, gtx, &w.import_, icons.Download, "Import from ~/.ssh/config", "") }),
		}

		if len(recent) > 0 {
			children = append(children, layout.Rigid(kit.Gap(18)))

			for _, h := range recent {
				children = append(children, layout.Rigid(func(gtx C) D { return w.recentRow(a, gtx, h) }))
			}
		}

		children = append(children,
			layout.Rigid(kit.Gap(18)),
			layout.Rigid(func(gtx C) D { return w.action(a, gtx, &w.commands, nil, "Show all commands", "Ctrl+Shift+P") }),
		)

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (w *Screen) action(a core.Host, gtx C, r *kit.Row, ic *icons.Icon, label, hint string) D {
	k := a.Kit()
	th := k.Th
	hv := k.Hover(gtx, kit.Key(r, 9), r.Hovered())
	fg := kit.Lerp(th.TextMuted, th.Text, hv)

	return layout.Inset{Left: -12, Right: -12}.Layout(gtx, func(gtx C) D {
		return k.Row(gtx, r, kit.RowOpts{Height: 30}, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if ic == nil {
						return D{}
					}

					return layout.Inset{Right: 10}.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 16, kit.Lerp(th.TextFaint, th.Text, hv)) })
				}),
				layout.Flexed(1, func(gtx C) D { return k.Label(gtx, kit.TextStyle{Color: fg, MaxLines: 1}, label) }),
				layout.Rigid(func(gtx C) D {
					if hint == "" {
						return D{}
					}

					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, hint)
				}),
			)
		})
	})
}

func (w *Screen) recentRow(a core.Host, gtx C, h vault.Host) D {
	k := a.Kit()
	th := a.Kit().Th

	if w.recent == nil {
		w.recent = map[string]*kit.Row{}
	}

	r := w.recent[h.ID]

	if r == nil {
		r = &kit.Row{}
		w.recent[h.ID] = r
	}

	if ev := r.Update(gtx); ev.Click {
		a.Connect(h)
	}

	return layout.Inset{Left: -12, Right: -12}.Layout(gtx, func(gtx C) D {
		hv := k.Hover(gtx, kit.Key(r, 9), r.Hovered())
		fg := kit.Lerp(th.TextMuted, th.Text, hv)
		return k.Row(gtx, r, kit.RowOpts{Height: 30}, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icons.Terminal.Layout(gtx, 16, kit.Lerp(th.TextFaint, th.Text, hv)) }),
				layout.Rigid(kit.Gap(10)),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Color: kit.Lerp(fg, th.Text, 0.6)}, h.Title()) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint, MaxLines: 1}, h.Target())
				}),
				layout.Rigid(func(gtx C) D {
					if h.LastUsed.IsZero() {
						return D{}
					}

					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, ago(time.Since(h.LastUsed)))
				}),
			)
		})
	})
}

func greeting(t time.Time, name string) string {
	var s string

	switch h := t.Hour(); {
	case h < 6:
		if name != "" {
			return "Up late, " + name + "?"
		}

		return "Up late?"
	case h < 13:
		s = "Good morning"
	case h < 21:
		s = "Good afternoon"
	default:
		s = "Good evening"
	}

	if name != "" {
		s += ", " + name
	}

	return s
}

func userName() string {
	n := strings.TrimSpace(os.Getenv("USERNAME"))

	if n == "" || strings.EqualFold(n, "user") || strings.EqualFold(n, "usuario") {
		return ""
	}

	r := []rune(n)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func summary(a core.Host) string {
	d := a.Vault().Snapshot()
	hosts, folders := len(d.Hosts), len(d.AllFolders())

	if hosts == 0 {
		return "Add your first server or open a local terminal."
	}

	s := plural(hosts, "saved server", "saved servers")

	if folders > 0 {
		s += " in " + plural(folders, "folder", "folders")
	}

	if n := a.Tunnels().Active(); n > 0 {
		s += " · " + plural(n, "active tunnel", "active tunnels")
	}

	return s + "."
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}

	return strconv.Itoa(n) + " " + many
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + " min ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + " h ago"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return strconv.Itoa(int(d.Hours()/24)) + " days ago"
	}
}

func New(h core.Host) *Screen {
	return &Screen{h: h}
}

type (
	C = core.C
	D = core.D
)
