package keys

import (
	"image/color"
	"io"
	"strconv"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const mimeText = "application/text"

type List struct {
	h     ui.Host
	list  widget.List
	rows  map[string]*widget.Clickable
	add   widget.Clickable
	empty widget.Clickable
	count int
}

func NewList(h ui.Host) *List {
	return &List{h: h, rows: map[string]*widget.Clickable{}}
}

func (l *List) Title() string { return "Keys" }

func (l *List) Subtitle() string {
	switch l.count {
	case 0:
		return ""
	case 1:
		return "1 key"
	}

	return strconv.Itoa(l.count) + " keys"
}

func (l *List) Actions() []ui.Action {
	if l.count == 0 {
		return nil
	}

	return []ui.Action{{Icon: icons.Add, Click: &l.add, Do: func() { l.h.ShowSheet(addSheet(l.h)) }}}
}

func (l *List) Layout(gtx C) D {
	k := l.h.Kit()
	all := l.h.Data().Keys
	l.count = len(all)

	if l.add.Clicked(gtx) || l.empty.Clicked(gtx) {
		l.h.ShowSheet(addSheet(l.h))
	}

	for _, kk := range all {
		if c := l.rows[kk.ID]; c != nil && c.Clicked(gtx) {
			l.h.Push(NewDetail(l.h, kk.ID))
		}
	}

	if len(all) == 0 {
		return ui.Empty(gtx, k, icons.LucideKey, "No SSH keys",
			"Sign in without typing a password. Generate a key here and add its public half to the server, or import one you already use.",
			func(gtx C) D { return ui.Button(gtx, k, &l.empty, ui.Primary, icons.Add, "Add key", true) })
	}

	rows := make([]ui.W, len(all))

	for i, kk := range all {
		clk := l.rows[kk.ID]

		if clk == nil {
			clk = new(widget.Clickable)
			l.rows[kk.ID] = clk
		}

		rows[i] = func(gtx C) D {
			return ui.Row(gtx, k, clk, ui.RowOpts{
				Lead:     ui.TileFor(k, icons.LucideKey, color.NRGBA{}),
				Title:    kk.Label,
				Subtitle: describe(kk),
				Chevron:  true,
			})
		}
	}

	return ui.Page(gtx, &l.list, func(gtx C) D { return ui.Section(gtx, k, "", rows...) })
}

func addSheet(h ui.Host) *ui.Sheet {
	return &ui.Sheet{
		Title: "Add a key",
		Items: []ui.SheetItem{
			{Icon: icons.LucideKey, Label: "Generate a new key", Do: func() { h.Push(NewGenerate(h)) }},
			{Icon: icons.Paste, Label: "Import a private key", Do: func() { h.Push(NewImport(h)) }},
		},
	}
}

func describe(k vault.Key) string {
	s := KeyType(k)

	if fp := sshx.Fingerprint(k.Public); fp != "" {
		s += " · " + short(fp)
	}

	return s
}

func KeyType(k vault.Key) string {
	return strings.ToUpper(strings.TrimPrefix(k.Type, "ssh-"))
}

func short(fp string) string {
	if len(fp) > 22 {
		return fp[:22] + "…"
	}

	return fp
}

type Generate struct {
	h    ui.Host
	name widget.Editor
	pass widget.Editor
	make widget.Clickable
	list widget.List
}

func NewGenerate(h ui.Host) *Generate {
	g := &Generate{h: h}
	ui.SetupField(&g.name, key.HintAny, false)
	ui.SetupField(&g.pass, key.HintPassword, true)
	g.name.SetText("Phone key")
	return g
}

func (g *Generate) Title() string { return "New key" }

func (g *Generate) Actions() []ui.Action {
	return []ui.Action{{Icon: icons.Check, Click: &g.make, Do: g.generate, Primary: true}}
}

func (g *Generate) generate() {
	name := strings.TrimSpace(g.name.Text())

	if name == "" {
		name = "Phone key"
	}

	kk, err := sshx.GenerateEd25519(name, g.pass.Text())

	if err == nil {
		kk, err = g.h.Vault().SaveKey(kk)
	}

	if err != nil {
		g.h.Notify("Couldn't create the key", ui.Sentence(err))
		return
	}

	g.h.Pop()
	g.h.Push(NewDetail(g.h, kk.ID))
}

func (g *Generate) Layout(gtx C) D {
	k := g.h.Kit()

	return ui.Page(gtx, &g.list,
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D { return ui.FormRow(gtx, k, "Name", &g.name, ui.FormOpts{Hint: "Phone key"}) },
				func(gtx C) D { return ui.FormRow(gtx, k, "Passphrase", &g.pass, ui.FormOpts{Hint: "Optional"}) },
			)
		},
		func(gtx C) D {
			return ui.Caption(gtx, k, "Boreal creates an Ed25519 key, the modern default for SSH. A passphrase encrypts the key itself, and you type it when you connect. Add the public key to ~/.ssh/authorized_keys on your servers.")
		},
	)
}

type Import struct {
	h     ui.Host
	name  widget.Editor
	data  widget.Editor
	pass  widget.Editor
	paste widget.Clickable
	save  widget.Clickable
	list  widget.List
	err   string
}

func NewImport(h ui.Host) *Import {
	im := &Import{h: h}
	ui.SetupField(&im.name, key.HintAny, false)
	ui.SetupField(&im.pass, key.HintPassword, true)
	im.data.InputHint = key.HintPassword
	return im
}

func (im *Import) Title() string { return "Import key" }

func (im *Import) Actions() []ui.Action {
	return []ui.Action{{Icon: icons.Check, Click: &im.save, Do: im.submit, Primary: true}}
}

func (im *Import) Layout(gtx C) D {
	k := im.h.Kit()

	for {
		ev, ok := gtx.Event(transfer.TargetFilter{Target: im, Type: mimeText})

		if !ok {
			break
		}

		if e, ok := ev.(transfer.DataEvent); ok {
			if r := e.Open(); r != nil {
				b, _ := io.ReadAll(io.LimitReader(r, 64<<10))
				r.Close()
				im.data.SetText(strings.TrimSpace(string(b)))
			}
		}
	}

	if im.paste.Clicked(gtx) {
		gtx.Execute(clipboard.ReadCmd{Tag: im})
	}

	event.Op(gtx.Ops, im)
	return ui.Page(gtx, &im.list,
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D { return ui.FormRow(gtx, k, "Name", &im.name, ui.FormOpts{Hint: "Work laptop key"}) },
			)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return ui.Section(gtx, k, "Private key",
						func(gtx C) D {
							return ui.FormArea(gtx, k, &im.data, ui.FormOpts{Hint: "-----BEGIN OPENSSH PRIVATE KEY-----", Mono: true})
						},
						func(gtx C) D { return ui.ActionRow(gtx, k, &im.paste, icons.Paste, "Paste from clipboard", ui.Primary) },
					)
				}),
				layout.Rigid(func(gtx C) D {
					if im.err == "" {
						return D{}
					}

					return layout.Inset{Left: 4, Top: 8}.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: k.Th.Danger, MaxLines: 3}, im.err)
					})
				}),
			)
		},
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D {
					return ui.FormRow(gtx, k, "Passphrase", &im.pass, ui.FormOpts{Hint: "If the key has one"})
				},
			)
		},
	)
}

func (im *Import) submit() {
	name := strings.TrimSpace(im.name.Text())

	if name == "" {
		name = "Imported key"
	}

	kk, err := sshx.ImportKey(name, []byte(strings.TrimSpace(im.data.Text())), im.pass.Text())

	if err != nil {
		im.err = "This doesn't look like a private key, or the passphrase is wrong."
		return
	}

	kk, err = im.h.Vault().SaveKey(kk)

	if err != nil {
		im.h.Notify("Couldn't save the key", ui.Sentence(err))
		return
	}

	im.h.Pop()
	im.h.Push(NewDetail(im.h, kk.ID))
}

type Detail struct {
	h      ui.Host
	id     string
	copy   widget.Clickable
	remove widget.Clickable
	list   widget.List
}

func NewDetail(h ui.Host, id string) *Detail {
	return &Detail{h: h, id: id}
}

func (d *Detail) Title() string {
	if kk, ok := d.h.Vault().Key(d.id); ok {
		return kk.Label
	}

	return "Key"
}

func (d *Detail) Layout(gtx C) D {
	k := d.h.Kit()
	th := k.Th
	kk, ok := d.h.Vault().Key(d.id)

	if !ok {
		return D{Size: gtx.Constraints.Max}
	}

	if d.copy.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: mimeText, Data: io.NopCloser(strings.NewReader(kk.Public))})
		d.h.Notify("Public key copied", "Paste it into ~/.ssh/authorized_keys on the server.")
	}

	if d.remove.Clicked(gtx) {
		d.confirmDelete(kk)
	}

	return ui.Page(gtx, &d.list,
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D { return ui.Row(gtx, k, nil, ui.RowOpts{Title: "Type", Value: KeyType(kk)}) },
				func(gtx C) D {
					return ui.Row(gtx, k, nil, ui.RowOpts{Title: "Fingerprint", Subtitle: sshx.Fingerprint(kk.Public), Mono: true})
				},
				func(gtx C) D {
					v := "No"

					if kk.Passphrase != "" || strings.Contains(kk.Private, "ENCRYPTED") {
						v = "Yes"
					}

					return ui.Row(gtx, k, nil, ui.RowOpts{Title: "Passphrase", Value: v})
				},
			)
		},
		func(gtx C) D {
			return ui.Section(gtx, k, "Public key",
				func(gtx C) D {
					return layout.Inset{Left: ui.Gutter, Right: ui.Gutter, Top: 14, Bottom: 14}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: th.TextMuted, Mono: true, MaxLines: 12}, kk.Public)
					})
				},
				func(gtx C) D { return ui.ActionRow(gtx, k, &d.copy, icons.Copy, "Copy public key", ui.Primary) },
			)
		},
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D { return ui.ActionRow(gtx, k, &d.remove, icons.Delete, "Delete key", ui.Destructive) },
			)
		},
	)
}

func (d *Detail) confirmDelete(kk vault.Key) {
	used := 0

	for _, h := range d.h.Data().Hosts {
		if h.KeyID == kk.ID {
			used++
		}
	}

	msg := "The private key is erased from this device. Servers that only accept this key will refuse the connection."

	if used > 0 {
		msg = strconv.Itoa(used) + " host(s) use this key and will ask for a password instead. " + msg
	}

	d.h.Ask(&ui.Dialog{
		Title:   "Delete " + kk.Label + "?",
		Message: msg,
		Confirm: "Delete key",
		Cancel:  "Cancel",
		Danger:  true,
		Reply: func(ok bool, _ string) {
			if !ok {
				return
			}

			if err := d.h.Vault().DeleteKey(kk.ID); err != nil {
				d.h.Notify("Couldn't delete the key", ui.Sentence(err))
				return
			}

			d.h.Pop()
		},
	})
}
