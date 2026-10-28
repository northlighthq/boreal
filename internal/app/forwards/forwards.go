package forwards

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/tunnel"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	fwdMenuToggle = iota
	fwdMenuEdit
	fwdMenuSep
	fwdMenuDelete
)

type Panel struct {
	h core.Host

	add      widget.Clickable
	emptyAdd widget.Clickable
	list     kit.ScrollList
	menu     kit.Menu
	rows     map[string]*fwdRow
	rules    []vault.Forward
	hosts    map[string]vault.Host
	pick     vault.Forward
}

type fwdRow struct {
	row    kit.Row
	toggle widget.Bool
}

func (p *Panel) Title() string     { return "Port Forwarding" }
func (p *Panel) Icon() *icons.Icon { return icons.ArrowsOutline }
func (p *Panel) Side() core.Side   { return core.Right }
func (p *Panel) Shortcut() string  { return "Alt+3" }

func (p *Panel) Refresh() {
	a := p.h
	d := a.Vault().Snapshot()
	p.rules = d.Forwards
	p.hosts = map[string]vault.Host{}

	for _, h := range d.Hosts {
		p.hosts[h.ID] = h
	}
}

func (p *Panel) Actions(gtx C) D {
	a := p.h

	if p.add.Clicked(gtx) {
		a.OpenIn(p, NewEditor(a, vault.Forward{Kind: vault.ForwardLocal}))
	}

	return a.Kit().IconButtonSized(gtx, &p.add, icons.Add, false, 26, 18, a.Kit().Th.TextMuted)
}

func (p *Panel) Body(gtx C) D {
	a := p.h
	k := a.Kit()
	th := a.Kit().Th

	if p.rows == nil {
		p.rows = map[string]*fwdRow{}
	}

	p.handleMenu(a)

	if len(p.rules) == 0 {
		if p.emptyAdd.Clicked(gtx) {
			a.OpenIn(p, NewEditor(a, vault.Forward{Kind: vault.ForwardLocal}))
		}

		return a.Kit().EmptyState(gtx, icons.Forward, "No rules",
			"Create local or remote tunnels, or a SOCKS5 proxy, through your hosts.",
			func(gtx C) D { return k.Button(gtx, &p.emptyAdd, kit.Primary, icons.Add, "New rule") },
		)
	}

	dims := k.List(gtx, &p.list, len(p.rules), func(gtx C, i int) D {
		r := p.rules[i]
		fr := p.rows[r.ID]

		if fr == nil {
			fr = &fwdRow{}
			p.rows[r.ID] = fr
		}

		st, running := a.Tunnels().State(r.ID)
		fr.toggle.Value = running && (st.Tunnel != nil || st.Starting)

		if fr.toggle.Update(gtx) {
			if fr.toggle.Value {
				a.Tunnels().Start(r)
			} else {
				a.Tunnels().Stop(r.ID)
			}
		}

		if ev := fr.row.Update(gtx); ev.Context {
			p.pick = r
			toggle := "Start"

			if running {
				toggle = "Stop"
			}

			p.menu.Open([]kit.MenuItem{
				fwdMenuToggle: {Label: toggle, Icon: icons.Play},
				fwdMenuEdit:   {Label: "Edit…", Icon: icons.Edit},
				fwdMenuSep:    kit.Separator(),
				fwdMenuDelete: {Label: "Delete", Icon: icons.Delete, Danger: true},
			}, ev.At.Add(image.Pt(0, gtx.Dp(4))))
		}

		host := p.hosts[r.HostID].Title()
		status, color := "Stopped", th.TextFaint

		switch {
		case st.Starting:
			status, color = "Connecting…", th.TextMuted
		case st.Tunnel != nil:
			status, color = "Active", th.Text
		}

		return k.Row(gtx, &fr.row, kit.RowOpts{Height: 74}, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return kit.Dot(gtx, color, 8) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Weight: font.Medium}, tunnel.Label(r)) }),
						layout.Rigid(kit.Gap(4)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: th.TextFaint, Mono: true}, sshx.Describe(r))
						}),
						layout.Rigid(kit.Gap(3)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: th.TextMuted}, "via "+host+" · "+status)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D { return k.Toggle(gtx, &fr.toggle) }),
			)
		})
	})

	k.Menu(gtx, &p.menu)
	return dims
}

func (p *Panel) handleMenu(a core.Host) {
	i, ok := p.menu.Chosen()

	if !ok {
		return
	}

	r := p.pick

	switch i {
	case fwdMenuToggle:
		if _, running := a.Tunnels().State(r.ID); running {
			a.Tunnels().Stop(r.ID)
		} else {
			a.Tunnels().Start(r)
		}
	case fwdMenuEdit:
		a.OpenIn(p, NewEditor(a, r))
	case fwdMenuDelete:
		a.Open(dialogs.NewConfirm(a.Kit(), "Delete this rule?", "“"+tunnel.Label(r)+"” will stop forwarding and be removed.", "Delete", func() {
			a.Tunnels().Stop(r.ID)
			a.Vault().DeleteForward(r.ID)
		}))
	}
}

var forwardKinds = []string{vault.ForwardLocal, vault.ForwardRemote, vault.ForwardDynamic}

type Editor struct {
	h       core.Host
	rule    vault.Forward
	dlg     kit.Dialog
	kind    kit.Segmented
	host    kit.Select
	hostIDs []string
	names   []string

	label, bindAddr, bindPort, destHost, destPort widget.Editor
	save, cancel                                  widget.Clickable
	invalid                                       bool
}

func NewEditor(a core.Host, r vault.Forward) *Editor {
	e := &Editor{h: a, rule: r}
	dialogs.SingleLine(&e.label, &e.bindAddr, &e.bindPort, &e.destHost, &e.destPort)
	e.bindPort.Filter = "0123456789"
	e.destPort.Filter = "0123456789"

	e.label.SetText(r.Label)
	e.bindAddr.SetText(r.BindAddr)

	if r.BindPort > 0 {
		e.bindPort.SetText(strconv.Itoa(r.BindPort))
	}

	e.destHost.SetText(r.DestHost)

	if r.DestPort > 0 {
		e.destPort.SetText(strconv.Itoa(r.DestPort))
	}

	for i, k := range forwardKinds {
		if k == r.Kind {
			e.kind.Value = i
		}
	}

	for _, h := range a.Vault().Snapshot().Hosts {
		e.hostIDs = append(e.hostIDs, h.ID)
		e.names = append(e.names, h.Title())

		if h.ID == r.HostID {
			e.host.Value = len(e.hostIDs) - 1
		}
	}

	return e
}

func (e *Editor) submit(a core.Host) bool {
	if len(e.hostIDs) == 0 {
		return false
	}

	r := e.rule
	r.Kind = forwardKinds[e.kind.Value]
	r.HostID = e.hostIDs[e.host.Value]
	r.Label = strings.TrimSpace(e.label.Text())
	r.BindAddr = strings.TrimSpace(e.bindAddr.Text())
	r.BindPort, _ = strconv.Atoi(e.bindPort.Text())
	r.DestHost = strings.TrimSpace(e.destHost.Text())
	r.DestPort, _ = strconv.Atoi(e.destPort.Text())

	if r.BindPort <= 0 || (r.Kind != vault.ForwardDynamic && (r.DestHost == "" || r.DestPort <= 0)) {
		e.invalid = true
		return false
	}

	if r.Kind == vault.ForwardDynamic {
		r.DestHost, r.DestPort = "", 0
	}

	a.Vault().SaveForward(r)
	return true
}

func (e *Editor) Layout(gtx C) bool {
	a := e.h
	k := a.Kit()
	th := a.Kit().Th

	if e.save.Clicked(gtx) && e.submit(a) {
		return true
	}

	if e.cancel.Clicked(gtx) {
		return true
	}

	title := "New rule"

	if e.rule.ID != "" {
		title = "Edit rule"
	}

	descs := []string{
		"Listens on your machine and forwards to the destination through the server (ssh -L).",
		"Listens on the server and forwards to a destination reachable from your machine (ssh -R).",
		"Local SOCKS5 proxy that reaches the Internet through the server (ssh -D).",
	}

	row := func(label string, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx C) D {
				return k.FormRow(gtx, label, dialogs.LabelWidth, w)
			})
		})
	}

	port := func(ed *widget.Editor, hint string) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(90)
			gtx.Constraints.Max.X = gtx.Dp(90)
			return k.Field(gtx, ed, kit.FieldOpts{Hint: hint, Invalid: e.invalid && ed.Text() == ""})
		}
	}

	children := []layout.FlexChild{
		row("Type", func(gtx C) D { return k.Segmented(gtx, &e.kind, []string{"Local", "Remote", "Dynamic"}) }),
		layout.Rigid(func(gtx C) D {
			in := layout.Inset{Left: dialogs.LabelWidth, Bottom: 14}

			if k.Narrow() {
				in = layout.Inset{Top: -4, Bottom: 14}
			}

			return in.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 4}, descs[e.kind.Value])
			})
		}),
		row("Host", func(gtx C) D {
			if len(e.names) == 0 {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 2}, "No saved hosts yet. Save one first, then add the rule here.")
			}

			return k.Select(gtx, &e.host, e.names)
		}),
		row("Name", func(gtx C) D { return k.Field(gtx, &e.label, kit.FieldOpts{Hint: "Production Postgres"}) }),
		row("Listen on", func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { return k.Field(gtx, &e.bindAddr, kit.FieldOpts{Hint: "127.0.0.1"}) }),
				layout.Rigid(kit.Gap(8)),
				layout.Rigid(func(gtx C) D { return k.Muted(gtx, ":") }),
				layout.Rigid(kit.Gap(8)),
				layout.Rigid(port(&e.bindPort, "5432")),
			)
		}),
	}

	if e.kind.Value != 2 {
		children = append(children, row("Destination", func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return k.Field(gtx, &e.destHost, kit.FieldOpts{Hint: "localhost", Invalid: e.invalid && e.destHost.Text() == ""})
				}),
				layout.Rigid(kit.Gap(8)),
				layout.Rigid(func(gtx C) D { return k.Muted(gtx, ":") }),
				layout.Rigid(kit.Gap(8)),
				layout.Rigid(port(&e.destPort, "5432")),
			)
		}))
	}

	return k.Dialog(gtx, &e.dlg, kit.DialogOpts{
		Title:    title,
		Subtitle: "Send traffic for a port through one of your SSH hosts.",
		Width:    560,
		Body: func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &e.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &e.save, kit.Primary, nil, "Save") },
		),
	})
}

func (e *Editor) Dialog() *kit.Dialog {
	return &e.dlg
}

func New(h core.Host) *Panel {
	return &Panel{h: h}
}

type (
	C = core.C
	D = core.D
)
