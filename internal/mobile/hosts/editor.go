package hosts

import (
	"image/color"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/keys"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const (
	authPassword = iota
	authKey
)

var authLabels = []string{"Password", "SSH key"}

type Editor struct {
	h     ui.Host
	host  vault.Host
	isNew bool

	label    widget.Editor
	address  widget.Editor
	port     widget.Editor
	user     widget.Editor
	password widget.Editor
	folder   widget.Editor
	startup  widget.Editor
	auth     ui.Segmented
	keyID    string
	keyRows  map[string]*widget.Clickable
	reveal   widget.Clickable
	shown    bool

	save    widget.Clickable
	saveBar widget.Clickable
	remove  widget.Clickable
	list    widget.List
	tried   bool
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

	return e
}

func (e *Editor) Title() string {
	if e.isNew {
		return "New host"
	}

	return "Edit host"
}

func (e *Editor) Actions() []ui.Action {
	return []ui.Action{{Icon: icons.Check, Click: &e.save, Do: e.submit}}
}

func (e *Editor) Layout(gtx C) D {
	k := e.h.Kit()
	th := k.Th

	if e.saveBar.Clicked(gtx) {
		e.submit()
	}

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

	addrErr := e.tried && strings.TrimSpace(e.address.Text()) == ""
	portErr := e.portError()
	saved := e.h.Vault().Snapshot().Keys

	for _, kk := range saved {
		if c := e.keyRows[kk.ID]; c != nil && c.Clicked(gtx) {
			e.keyID = kk.ID
		}
	}

	children := []ui.W{
		func(gtx C) D {
			return ui.Field(gtx, k, &e.label, ui.FieldOpts{Label: "Name", Hint: "Production web"})
		},
		func(gtx C) D {
			help := ""

			if addrErr {
				help = "Enter the server's address or name."
			}

			return ui.Field(gtx, k, &e.address, ui.FieldOpts{Label: "Address", Hint: "203.0.113.10 or server.example.com", Invalid: addrErr, Help: help})
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &e.user, ui.FieldOpts{Label: "Username", Hint: "root"})
		},
		func(gtx C) D {
			help := ""

			if portErr {
				help = "Use a number between 1 and 65535."
			}

			return ui.Field(gtx, k, &e.port, ui.FieldOpts{Label: "Port", Hint: "22", Invalid: portErr, Help: help})
		},
		func(gtx C) D { return e.auth.Layout(gtx, k, authLabels) },
	}

	if e.auth.Value == authPassword {
		children = append(children, func(gtx C) D {
			ic := icons.Visibility

			if e.shown {
				ic = icons.VisibilityOff
			}

			return ui.Field(gtx, k, &e.password, ui.FieldOpts{
				Label:    "Password",
				Hint:     "Leave empty to be asked when you connect",
				Trailing: func(gtx C) D { return ui.IconButton(gtx, k, &e.reveal, ic, th.TextMuted) },
			})
		})
	} else {
		children = append(children, func(gtx C) D { return e.keyPicker(gtx, saved) })
	}

	children = append(children,
		func(gtx C) D {
			return ui.Field(gtx, k, &e.folder, ui.FieldOpts{Label: "Folder", Hint: "Optional, groups hosts in the list"})
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &e.startup, ui.FieldOpts{Label: "Run on connect", Hint: "Optional command", Mono: true})
		},
		func(gtx C) D {
			return ui.Button(gtx, k, &e.saveBar, ui.Primary, nil, "Save host", true)
		},
	)

	if !e.isNew {
		children = append(children, func(gtx C) D {
			return ui.Button(gtx, k, &e.remove, ui.Destructive, icons.Delete, "Delete host", true)
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

	return ui.Section(gtx, k, "KEY", rows...)
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

	if e.auth.Value == authKey {
		h.Auth, h.KeyID, h.Password = vault.AuthKey, e.keyID, ""
	} else {
		h.Auth, h.KeyID, h.Password = vault.AuthPassword, "", e.password.Text()
	}

	if _, err := e.h.Vault().SaveHost(h); err != nil {
		e.h.Notify("Couldn't save the host", err.Error())
		return
	}

	e.h.Pop()
}

func Icon(h vault.Host) (*icons.Icon, color.NRGBA) {
	if b, ok := icons.OS(h.OS); ok {
		return b.Icon, b.Color
	}

	return icons.LucideServer, color.NRGBA{}
}
