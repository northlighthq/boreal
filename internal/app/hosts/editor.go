package hosts

import (
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

type Editor struct {
	h    core.Host
	host vault.Host
	dlg  kit.Dialog

	label, address, port, user, password, group, tags, startup widget.Editor
	forward                                                    widget.Bool

	auth     kit.Segmented
	key      kit.Select
	jump     kit.Select
	reveal   widget.Clickable
	showPass bool

	keyIDs  []string
	jumpIDs []string

	save, saveConnect widget.Clickable
	invalid           bool
	keyMissing        bool
	focused           bool
	more              widget.Clickable
	expanded          bool
	list              layout.List
}

var authMethods = []string{vault.AuthPassword, vault.AuthKey, vault.AuthAgent}

func NewEditor(a core.Host, h vault.Host) *Editor {
	e := &Editor{h: a, host: h}
	dialogs.SingleLine(&e.label, &e.address, &e.port, &e.user, &e.password, &e.group, &e.tags, &e.startup)
	e.port.Filter = "0123456789"
	e.password.Mask = '•'

	e.label.SetText(h.Label)
	e.address.SetText(h.Address)

	if h.Port == 0 {
		h.Port = 22
	}

	e.port.SetText(strconv.Itoa(h.Port))
	e.user.SetText(h.Username)
	e.password.SetText(h.Password)
	e.group.SetText(h.Group)
	e.tags.SetText(strings.Join(h.Tags, ", "))
	e.startup.SetText(h.StartupCommand)
	e.forward.Value = h.ForwardAgent

	e.auth.Value = 2

	for i, m := range authMethods {
		if m == h.Auth {
			e.auth.Value = i
		}
	}

	if h.ID == "" {
		e.auth.Value = 0
	}

	d := a.Vault().Snapshot()
	e.keyIDs = []string{""}

	for _, k := range d.Keys {
		e.keyIDs = append(e.keyIDs, k.ID)

		if k.ID == h.KeyID {
			e.key.Value = len(e.keyIDs) - 1
		}
	}

	e.jumpIDs = []string{""}

	for _, x := range d.Hosts {
		if x.ID == h.ID {
			continue
		}

		e.jumpIDs = append(e.jumpIDs, x.ID)

		if x.ID == h.JumpID {
			e.jump.Value = len(e.jumpIDs) - 1
		}
	}

	e.expanded = h.Group != "" || len(h.Tags) > 0 || h.JumpID != "" || h.StartupCommand != "" || h.ForwardAgent
	e.list.Axis = layout.Vertical
	return e
}

func (e *Editor) collect() (vault.Host, bool) {
	h := e.host
	h.Label = strings.TrimSpace(e.label.Text())
	h.Address = strings.TrimSpace(e.address.Text())
	h.Port, _ = strconv.Atoi(e.port.Text())
	h.Username = strings.TrimSpace(e.user.Text())
	h.Auth = authMethods[e.auth.Value]
	h.Password = ""

	if h.Auth == vault.AuthPassword {
		h.Password = e.password.Text()
	}

	h.KeyID = ""

	if h.Auth == vault.AuthKey {
		h.KeyID = e.keyIDs[e.key.Value]
	}

	h.JumpID = e.jumpIDs[e.jump.Value]
	h.StartupCommand = strings.TrimSpace(e.startup.Text())
	h.ForwardAgent = e.forward.Value
	h.Group = vault.CleanFolder(e.group.Text())
	h.Tags = nil

	for _, t := range strings.Split(e.tags.Text(), ",") {
		if t = strings.TrimSpace(t); t != "" {
			h.Tags = append(h.Tags, t)
		}
	}

	if h.Address == "" || h.Username == "" {
		return h, false
	}

	if h.Auth == vault.AuthKey && h.KeyID == "" {
		e.keyMissing = true
		return h, false
	}

	if h.Port <= 0 || h.Port > 65535 {
		h.Port = 22
	}

	return h, true
}

func (e *Editor) submit(a core.Host, connect bool) bool {
	h, ok := e.collect()

	if !ok {
		e.invalid = true
		return false
	}

	saved, err := a.Vault().SaveHost(h)

	if err != nil {
		a.Toasts().Error("Couldn't save", err.Error())
		return false
	}

	if connect {
		a.Connect(saved)
	}

	return true
}

func (e *Editor) Layout(gtx C) bool {
	a := e.h
	k := a.Kit()
	th := a.Kit().Th

	if e.save.Clicked(gtx) && e.submit(a, false) {
		return true
	}

	if e.saveConnect.Clicked(gtx) && e.submit(a, true) {
		return true
	}

	if e.reveal.Clicked(gtx) {
		e.showPass = !e.showPass
		e.password.Mask = '•'

		if e.showPass {
			e.password.Mask = 0
		}
	}

	for _, ed := range []*widget.Editor{&e.label, &e.address, &e.port, &e.user, &e.password, &e.group, &e.tags, &e.startup} {
		for {
			ev, ok := ed.Update(gtx)

			if !ok {
				break
			}

			if _, ok := ev.(widget.SubmitEvent); ok && e.submit(a, false) {
				return true
			}
		}
	}

	if !e.focused {
		gtx.Execute(key.FocusCmd{Tag: &e.address})

		if e.host.ID != "" {
			gtx.Execute(key.FocusCmd{Tag: &e.label})
		}

		e.focused = true
	}

	title := "New host"

	if e.host.ID != "" {
		title = "Edit host"
	}

	keyNames := []string{"Choose a key…"}
	jumpNames := []string{"None (direct connection)"}
	d := a.Vault().Snapshot()

	for _, id := range e.keyIDs[1:] {
		for _, kk := range d.Keys {
			if kk.ID == id {
				keyNames = append(keyNames, kk.Label+"  ·  "+strings.ToUpper(strings.TrimPrefix(kk.Type, "ssh-")))
			}
		}
	}

	for _, id := range e.jumpIDs[1:] {
		for _, h := range d.Hosts {
			if h.ID == id {
				jumpNames = append(jumpNames, h.Title())
			}
		}
	}

	for len(keyNames) < len(e.keyIDs) {
		keyNames = append(keyNames, "?")
	}

	for len(jumpNames) < len(e.jumpIDs) {
		jumpNames = append(jumpNames, "?")
	}

	field := func(label string, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if label == "" {
							return D{}
						}

						return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, Weight: font.Medium}, label)
						})
					}),
					layout.Rigid(w),
				)
			})
		})
	}

	card := func(_ string, rows ...layout.FlexChild) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		})
	}

	if e.more.Clicked(gtx) {
		e.expanded = !e.expanded
	}

	connection := card("Connection",
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, field("Address", func(gtx C) D {
						return k.Field(gtx, &e.address, kit.FieldOpts{Hint: "example.com or 10.0.0.5", Invalid: e.invalid && strings.TrimSpace(e.address.Text()) == ""})
					}))
				}),
				layout.Rigid(kit.Gap(8)),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(72), gtx.Dp(72)
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, field("Port", func(gtx C) D {
						return k.Field(gtx, &e.port, kit.FieldOpts{Hint: "22"})
					}))
				}),
			)
		}),
		field("Username", func(gtx C) D {
			return k.Field(gtx, &e.user, kit.FieldOpts{Hint: "e.g. root", Invalid: e.invalid && strings.TrimSpace(e.user.Text()) == ""})
		}),
		field("Label", func(gtx C) D { return k.Field(gtx, &e.label, kit.FieldOpts{Hint: "Display name (optional)"}) }),
	)

	if e.auth.Value == 1 && e.key.Value == 0 && len(e.keyIDs) > 1 {
		e.key.Value = 1
	}

	if e.key.Value != 0 {
		e.keyMissing = false
	}

	authRows := []layout.FlexChild{
		field("Sign in with", func(gtx C) D {
			return k.Segmented(gtx, &e.auth, []string{"Password", "SSH key", "Agent"})
		}),
	}

	switch e.auth.Value {
	case 0:
		authRows = append(authRows, field("Password", func(gtx C) D {
			ic := icons.Visibility

			if e.showPass {
				ic = icons.VisibilityOff
			}

			return k.Field(gtx, &e.password, kit.FieldOpts{
				Hint: "Leave empty to ask when connecting",
				Trailing: func(gtx C) D {
					return k.IconButtonSized(gtx, &e.reveal, ic, false, 22, 16, th.TextMuted)
				},
			})
		}))
	case 1:
		authRows = append(authRows, field("Key", func(gtx C) D {
			if len(keyNames) == 1 {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 2}, "No keys yet. Add one in Keychain (Alt+2).")
			}

			if !e.keyMissing {
				return k.Select(gtx, &e.key, keyNames)
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return k.Select(gtx, &e.key, keyNames) }),
				layout.Rigid(kit.Gap(6)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger}, "Choose the key to use.")
				}),
			)
		}))
	case 2:
		authRows = append(authRows, field("", func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, agentHint)
		}))
	}

	moreToggle := layout.Rigid(func(gtx C) D {
		return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D {
			return e.more.Layout(gtx, func(gtx C) D {
				hv := k.Hover(gtx, kit.Key(&e.more, 0), e.more.Hovered())
				fg := kit.Lerp(th.TextMuted, th.Text, hv)
				ic := icons.ChevronRight

				if e.expanded {
					ic = icons.ExpandMore
				}

				return layout.Inset{Left: 4, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 16, fg) }),
						layout.Rigid(kit.Gap(6)),
						layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Color: fg, Weight: font.Medium}, "More options") }),
					)
				})
			})
		})
	})

	children := []layout.FlexChild{connection, card("Authentication", authRows...), moreToggle}

	if e.expanded {
		children = append(children,
			card("Organization",
				field("Folder", func(gtx C) D {
					return k.Field(gtx, &e.group, kit.FieldOpts{Hint: "Production/Web"})
				}),
				field("Tags", func(gtx C) D { return k.Field(gtx, &e.tags, kit.FieldOpts{Hint: "web, db"}) }),
			),
			card("Advanced",
				field("Jump host", func(gtx C) D { return k.Select(gtx, &e.jump, jumpNames) }),
				field("Run on connect", func(gtx C) D {
					return k.Field(gtx, &e.startup, kit.FieldOpts{Hint: "e.g. tmux attach", Mono: true})
				}),
				field("", func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return k.Checkbox(gtx, &e.forward, "Forward my SSH agent") }),
						layout.Rigid(kit.Gap(4)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, "Lets this server use your local keys. Only for servers you trust.")
						}),
					)
				}),
			),
		)
	}

	preview := strings.TrimSpace(e.user.Text())

	if addr := strings.TrimSpace(e.address.Text()); addr != "" {
		if preview != "" {
			preview += "@"
		}

		preview += addr
	}

	closed := k.Dialog(gtx, &e.dlg, kit.DialogOpts{
		Title: title,
		Width: 560,
		Body: func(gtx C) D {
			return e.list.Layout(gtx, 1, func(gtx C, _ int) D {
				all := append([]layout.FlexChild{layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 18}.Layout(gtx, func(gtx C) D {
						if preview == "" {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, "Fill in the address and username to connect.")
						}

						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, Mono: true, MaxLines: 1}, preview)
					})
				})}, children...)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, all...)
			})
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &e.save, kit.Ghost, nil, "Save") },
			func(gtx C) D { return k.Button(gtx, &e.saveConnect, kit.Primary, icons.Play, "Save and connect") },
		),
	})
	return closed
}

func (e *Editor) Dialog() *kit.Dialog {
	return &e.dlg
}

func Defaults() vault.Host {
	return vault.Host{Port: 22, Auth: vault.AuthPassword}
}

func (e *Editor) SetAuth(method int) {
	e.auth.Value = method
}

func (e *Editor) KeyIndex() int {
	return e.key.Value
}
