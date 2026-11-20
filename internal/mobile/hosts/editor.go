package hosts

import (
	"image/color"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/keys"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const (
	authPassword = iota
	authKey
)

var authLabels = []string{"Password", "SSH key"}

const agentNote = "This host signs in with the SSH agent on your computer. Choose Password or SSH key to sign in from this phone."

type Editor struct {
	h     ui.Host
	host  vault.Host
	isNew bool

	label     widget.Editor
	address   widget.Editor
	port      widget.Editor
	user      widget.Editor
	password  widget.Editor
	folder    widget.Editor
	startup   widget.Editor
	auth      ui.Segmented
	startAuth int
	authSet   bool
	keyID     string
	keyRows   map[string]*widget.Clickable
	reveal    widget.Clickable
	shown     bool

	save   widget.Clickable
	remove widget.Clickable
	list   widget.List
	tried  bool
}

func NewEditor(h ui.Host, host vault.Host) *Editor {
	e := &Editor{h: h, host: host, isNew: host.ID == "", keyID: host.KeyID, keyRows: map[string]*widget.Clickable{}}
	ui.SetupField(&e.label, key.HintText, false)
	ui.SetupField(&e.address, key.HintURL, false)
	ui.SetupField(&e.port, key.HintNumeric, false)
	ui.SetupField(&e.user, key.HintEmail, false)
	ui.SetupField(&e.password, key.HintPassword, true)
	ui.SetupField(&e.folder, key.HintText, false)
	ui.SetupField(&e.startup, key.HintText, false)

	e.label.SetText(host.Label)
	e.address.SetText(host.Address)
	e.user.SetText(host.Username)
	e.password.SetText(host.Password)
	e.folder.SetText(host.Group)
	e.startup.SetText(host.StartupCommand)

	if host.Port != 0 && host.Port != 22 {
		e.port.SetText(strconv.Itoa(host.Port))
	}

	if host.Auth == vault.AuthKey {
		e.auth.Value = authKey
	}

	e.startAuth = e.auth.Value
	return e
}

func (e *Editor) keepsAgent() bool {
	return e.host.Auth == vault.AuthAgent && !e.authSet && e.password.Text() == e.host.Password
}

func authFor(keepAgent bool, choice int, keyID, password string) (string, string, string) {
	switch {
	case keepAgent:
		return vault.AuthAgent, "", ""
	case choice == authKey:
		return vault.AuthKey, keyID, ""
	}

	return vault.AuthPassword, "", password
}

func (e *Editor) Title() string {
	if e.isNew {
		return "New host"
	}

	return "Edit host"
}

func (e *Editor) Actions() []ui.Action {
	return []ui.Action{{Icon: icons.Check, Click: &e.save, Do: e.submit, Primary: true}}
}

func (e *Editor) Layout(gtx C) D {
	k := e.h.Kit()
	th := k.Th

	if e.reveal.Clicked(gtx) {
		e.shown = !e.shown

		if e.shown {
			e.password.Mask = 0
		} else {
			e.password.Mask = '•'
		}
	}

	if e.remove.Clicked(gtx) {
		confirmDelete(e.h, e.host, e.h.Pop)
	}

	if e.auth.Value != e.startAuth {
		e.authSet = true
	}

	addrErr := e.tried && strings.TrimSpace(e.address.Text()) == ""
	portErr := e.portError()
	saved := e.h.Data().Keys

	for _, kk := range saved {
		if c := e.keyRows[kk.ID]; c != nil && c.Clicked(gtx) {
			e.keyID = kk.ID
		}
	}

	connection := func(gtx C) D {
		return ui.Section(gtx, k, "Connection",
			func(gtx C) D { return ui.FormRow(gtx, k, "Name", &e.label, ui.FormOpts{Hint: "Production web"}) },
			func(gtx C) D {
				return ui.FormRow(gtx, k, "Address", &e.address, ui.FormOpts{Hint: "server.example.com", Mono: true, Invalid: addrErr})
			},
			func(gtx C) D { return ui.FormRow(gtx, k, "Port", &e.port, ui.FormOpts{Hint: "22", Mono: true, Invalid: portErr}) },
			func(gtx C) D { return ui.FormRow(gtx, k, "Username", &e.user, ui.FormOpts{Hint: "root", Mono: true}) },
		)
	}

	signIn := []ui.W{func(gtx C) D { return e.auth.Layout(gtx, k, authLabels) }}

	if e.keepsAgent() {
		signIn = append(signIn, func(gtx C) D { return ui.Caption(gtx, k, agentNote) })
	}

	if e.auth.Value == authPassword {
		signIn = append(signIn, func(gtx C) D {
			ic := icons.Visibility

			if e.shown {
				ic = icons.VisibilityOff
			}

			return ui.Section(gtx, k, "",
				func(gtx C) D {
					return ui.FormRow(gtx, k, "Password", &e.password, ui.FormOpts{
						Hint:     "Ask each time",
						Trailing: func(gtx C) D { return ui.IconButton(gtx, k, &e.reveal, ic, th.TextMuted) },
					})
				},
			)
		})
	} else {
		signIn = append(signIn, func(gtx C) D { return e.keyPicker(gtx, saved) })
	}

	children := []ui.W{connection}

	switch {
	case addrErr:
		children = append(children, func(gtx C) D { return ui.Caption(gtx, k, "Enter the server's address or name.") })
	case portErr:
		children = append(children, func(gtx C) D { return ui.Caption(gtx, k, "Use a port between 1 and 65535.") })
	}

	children = append(children,
		func(gtx C) D { return ui.Group(gtx, k, "Sign in", signIn...) },
		func(gtx C) D {
			return ui.Section(gtx, k, "Options",
				func(gtx C) D { return ui.FormRow(gtx, k, "Folder", &e.folder, ui.FormOpts{Hint: "None"}) },
				func(gtx C) D {
					return ui.FormRow(gtx, k, "On connect", &e.startup, ui.FormOpts{Hint: "cd /srv/app", Mono: true})
				},
			)
		},
	)

	if !e.isNew {
		children = append(children, func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D { return ui.ActionRow(gtx, k, &e.remove, icons.Delete, "Delete host", ui.Destructive) },
			)
		})
	}

	return ui.Page(gtx, &e.list, children...)
}

func (e *Editor) keyPicker(gtx C, saved []vault.Key) D {
	k := e.h.Kit()
	th := k.Th

	if len(saved) == 0 {
		return ui.Caption(gtx, k, "There are no SSH keys yet. Create or import one in the Keys tab, then pick it here.")
	}

	rows := make([]ui.W, len(saved))

	for i, kk := range saved {
		clk := e.keyRows[kk.ID]

		if clk == nil {
			clk = new(widget.Clickable)
			e.keyRows[kk.ID] = clk
		}

		rows[i] = func(gtx C) D {
			return ui.Row(gtx, k, clk, ui.RowOpts{
				Lead:     ui.TileFor(k, icons.LucideKey, color.NRGBA{}),
				Title:    kk.Label,
				Subtitle: keys.KeyType(kk),
				Trailing: func(gtx C) D {
					if kk.ID != e.keyID {
						return D{}
					}

					return icons.Check.Layout(gtx, 22, th.Accent)
				},
			})
		}
	}

	return ui.Section(gtx, k, "Key", rows...)
}

func (e *Editor) portError() bool {
	t := strings.TrimSpace(e.port.Text())

	if t == "" {
		return false
	}

	n, err := strconv.Atoi(t)
	return err != nil || n < 1 || n > 65535
}

func (e *Editor) submit() {
	e.tried = true
	addr := strings.TrimSpace(e.address.Text())

	if addr == "" || e.portError() {
		return
	}

	h := e.host
	h.Label = strings.TrimSpace(e.label.Text())
	h.Address = addr
	h.Username = strings.TrimSpace(e.user.Text())
	h.Group = strings.TrimSpace(e.folder.Text())
	h.StartupCommand = strings.TrimSpace(e.startup.Text())
	h.Port = 22

	if p := strings.TrimSpace(e.port.Text()); p != "" {
		h.Port, _ = strconv.Atoi(p)
	}

	h.Auth, h.KeyID, h.Password = authFor(e.keepsAgent(), e.auth.Value, e.keyID, e.password.Text())

	if _, err := e.h.Vault().SaveHost(h); err != nil {
		e.h.Notify("Couldn't save the host", err.Error())
		return
	}

	e.h.Pop()
}

func Icon(s config.Settings, h vault.Host) (*icons.Icon, color.NRGBA) {
	if s.HostIcons == config.HostIconsTerminal {
		return icons.Terminal, color.NRGBA{}
	}

	b, ok := icons.OS(h.OS)

	if !ok {
		return icons.Terminal, color.NRGBA{}
	}

	if s.HostIconTint == config.HostIconTintMono {
		return b.Icon, color.NRGBA{}
	}

	return b.Icon, b.Color
}
