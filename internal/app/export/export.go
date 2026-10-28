package export

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/backup"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

type (
	C = core.C
	D = core.D
)

type Dialog struct {
	h   core.Host
	dlg kit.Dialog

	hosts, keys, secrets, forwards, snippets, known widget.Bool

	password, confirm widget.Editor
	generate, reveal  widget.Clickable
	ok, cancel        widget.Clickable
	shown             bool
	focused           bool
	err               string
	list              layout.List
}

func New(h core.Host) *Dialog {
	d := &Dialog{h: h}
	d.hosts.Value, d.keys.Value, d.forwards.Value, d.snippets.Value, d.known.Value = true, true, true, true, true

	for _, e := range []*widget.Editor{&d.password, &d.confirm} {
		e.SingleLine, e.Submit, e.Mask = true, true, '•'
	}

	d.list.Axis = layout.Vertical
	return d
}

func (d *Dialog) Dialog() *kit.Dialog {
	return &d.dlg
}

func (d *Dialog) selection() backup.Selection {
	return backup.Selection{
		Hosts:      d.hosts.Value,
		Keys:       d.keys.Value,
		Secrets:    d.secrets.Value,
		Forwards:   d.forwards.Value && d.hosts.Value,
		Snippets:   d.snippets.Value,
		KnownHosts: d.known.Value,
	}
}

func (d *Dialog) submit() bool {
	pass := d.password.Text()
	s := d.selection()

	switch {
	case !s.Hosts && !s.Keys && !s.Snippets && !s.KnownHosts:
		d.err = "Choose something to export."
	case len([]rune(pass)) < backup.MinPassword:
		d.err = fmt.Sprintf("The password must be at least %d characters long.", backup.MinPassword)
	case pass != d.confirm.Text():
		d.err = "The passwords don't match."
	default:
		go write(d.h, d.h.Vault().Snapshot(), s, pass)
		return true
	}

	return false
}

func write(h core.Host, data vault.Data, s backup.Selection, pass string) {
	raw, err := backup.Export(data, s, pass)

	if err != nil {
		h.Toasts().Error("Couldn't export", err.Error())
		return
	}

	w, err := h.Explorer().CreateFile("platty-export" + backup.Extension)

	if err != nil {
		return
	}

	defer w.Close()

	if _, err := w.Write(raw); err != nil {
		h.Toasts().Error("Couldn't save the export", err.Error())
		return
	}

	h.Toasts().Success("Data exported", "Keep the file and its password in separate places.")
}

func (d *Dialog) Layout(gtx C) bool {
	k := d.h.Kit()
	th := k.Th

	if d.cancel.Clicked(gtx) {
		return true
	}

	submit := d.ok.Clicked(gtx)

	for _, e := range []*widget.Editor{&d.password, &d.confirm} {
		for {
			ev, ok := e.Update(gtx)

			if !ok {
				break
			}

			if _, ok := ev.(widget.SubmitEvent); ok {
				submit = true
			}
		}
	}

	if submit && d.submit() {
		return true
	}

	if d.generate.Clicked(gtx) {
		p := backup.GeneratePassword()
		d.password.SetText(p)
		d.confirm.SetText(p)
		d.shown, d.err = true, ""
	}

	if d.reveal.Clicked(gtx) {
		d.shown = !d.shown
	}

	mask := '•'

	if d.shown {
		mask = 0
	}

	d.password.Mask, d.confirm.Mask = mask, mask

	if !d.focused {
		gtx.Execute(key.FocusCmd{Tag: &d.password})
		d.focused = true
	}

	check := func(b *widget.Bool, label string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D { return k.Checkbox(gtx, b, label) })
		})
	}

	muted := func(s string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 4}, s)
		})
	}

	field := func(ed *widget.Editor, hint string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				ic := icons.Visibility

				if d.shown {
					ic = icons.VisibilityOff
				}

				return k.Field(gtx, ed, kit.FieldOpts{Icon: icons.Lock, Hint: hint, Mono: d.shown, Invalid: d.err != "", Trailing: func(gtx C) D {
					if ed != &d.password {
						return D{}
					}

					return k.IconButtonSized(gtx, &d.reveal, ic, false, 22, 16, th.TextMuted)
				}})
			})
		})
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "WHAT TO EXPORT") }),
		layout.Rigid(kit.Gap(10)),
		check(&d.hosts, "Hosts and folders"),
		check(&d.forwards, "Port forwarding rules"),
		check(&d.keys, "SSH keys (private keys included)"),
		check(&d.snippets, "Snippets"),
		check(&d.known, "Known hosts"),
		check(&d.secrets, "Host passwords and saved key passphrases"),
		layout.Rigid(kit.Gap(10)),
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "PASSWORD") }),
		layout.Rigid(kit.Gap(10)),
		field(&d.password, "Password for the file"),
		field(&d.confirm, "Repeat it"),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx C) D {
				return k.Button(gtx, &d.generate, kit.Secondary, icons.Key, "Generate a strong password")
			})
		}),
	}

	if d.err != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger}, d.err)
			})
		}))
	}

	children = append(children, muted("Anyone with the file and its password can read what's inside, so keep them apart. Boreal can't recover a lost password."))

	return k.Dialog(gtx, &d.dlg, kit.DialogOpts{
		Title:    "Export data",
		Subtitle: "Creates an encrypted .platty file you can import on another computer.",
		Width:    520,
		Body: func(gtx C) D {
			return d.list.Layout(gtx, 1, func(gtx C, _ int) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &d.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &d.ok, kit.Primary, icons.Download, "Export…") },
		),
	})
}

func Import(h core.Host) {
	f, err := h.Explorer().ChooseFile(backup.Extension)

	if err != nil {
		return
	}

	raw, err := io.ReadAll(f)
	f.Close()

	if err != nil {
		h.Toasts().Error("Couldn't read the file", err.Error())
		return
	}

	msg := "Enter the password this export was protected with."

	for {
		pass, ok := h.AskPassword("Import data", msg)

		if !ok {
			return
		}

		data, err := backup.Import(raw, pass)

		if errors.Is(err, backup.ErrWrongPassword) {
			msg = "That password doesn't open this file. Try again."
			continue
		}

		if err != nil {
			h.Toasts().Error("Couldn't import", err.Error())
			return
		}

		st, err := h.Vault().Merge(data)

		if err != nil {
			h.Toasts().Error("Couldn't import", err.Error())
			return
		}

		h.Toasts().Success("Data imported", summary(st))

		if st.Conflicts > 0 {
			h.Toasts().Info("Some host keys were kept", fmt.Sprintf("%d known hosts had a different key in the file; the ones you already trusted were kept.", st.Conflicts))
		}

		return
	}
}

func summary(st vault.MergeStats) string {
	var parts []string

	add := func(n int, one, many string) {
		switch {
		case n == 1:
			parts = append(parts, "1 "+one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}

	add(st.Hosts, "host", "hosts")
	add(st.Keys, "key", "keys")
	add(st.Forwards, "rule", "rules")
	add(st.Snippets, "snippet", "snippets")
	add(st.KnownHosts, "known host", "known hosts")

	if len(parts) == 0 {
		return "Nothing new in the file."
	}

	return strings.Join(parts, ", ")
}

func SSHConfig(h core.Host) {
	hosts := h.Vault().Snapshot().Hosts

	if len(hosts) == 0 {
		h.Toasts().Info("No hosts to export", "")
		return
	}

	w, err := h.Explorer().CreateFile("config")

	if err != nil {
		return
	}

	defer w.Close()

	if _, err := w.Write(backup.SSHConfig(hosts)); err != nil {
		h.Toasts().Error("Couldn't save the file", err.Error())
		return
	}

	h.Toasts().Success("SSH config exported", fmt.Sprintf("%d hosts, without passwords or keys.", len(hosts)))
}
