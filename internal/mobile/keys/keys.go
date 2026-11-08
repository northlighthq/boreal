package keys

import (
	"image/color"
	"io"
	"strconv"
	"strings"

	"gioui.org/font"
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

func (l *List) Layout(gtx C) D {
	k := l.h.Kit()
	all := l.h.Vault().Snapshot().Keys
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

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return ui.Page(gtx, &l.list, func(gtx C) D { return ui.Section(gtx, k, "", rows...) })
		}),
		layout.Expanded(func(gtx C) D { return ui.FAB(gtx, k, &l.add, icons.Add, "Add key") }),
	)
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
	ui.SetupField(&g.name, key.HintText, false)
	ui.SetupField(&g.pass, key.HintPassword, true)
	g.name.SetText("Phone key")
	return g
}

func (g *Generate) Title() string { return "New key" }

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
		g.h.Notify("Couldn't create the key", err.Error())
		return
	}

	g.h.Pop()
	g.h.Push(NewDetail(g.h, kk.ID))
}

func (g *Generate) Layout(gtx C) D {
	k := g.h.Kit()

	if g.make.Clicked(gtx) {
		g.generate()
	}

	return ui.Page(gtx, &g.list,
		func(gtx C) D {
			return ui.Field(gtx, k, &g.name, ui.FieldOpts{Label: "Name", Hint: "Phone key"})
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &g.pass, ui.FieldOpts{Label: "Passphrase", Hint: "Optional", Help: "Encrypts the key itself. You'll type it when you connect."})
		},
		func(gtx C) D {
			return ui.Caption(gtx, k, "Boreal creates an Ed25519 key, the modern default for SSH. Add its public key to ~/.ssh/authorized_keys on your servers.")
		},
		func(gtx C) D { return ui.Button(gtx, k, &g.make, ui.Primary, nil, "Generate key", true) },
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
	ui.SetupField(&im.name, key.HintText, false)
	ui.SetupField(&im.pass, key.HintPassword, true)
	im.data.InputHint = key.HintText
	return im
}

func (im *Import) Title() string { return "Import key" }

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

	if im.save.Clicked(gtx) {
		im.submit()
	}

	event.Op(gtx.Ops, im)
	return ui.Page(gtx, &im.list,
		func(gtx C) D {
			return ui.Field(gtx, k, &im.name, ui.FieldOpts{Label: "Name", Hint: "Work laptop key"})
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &im.data, ui.FieldOpts{Label: "Private key", Hint: "-----BEGIN OPENSSH PRIVATE KEY-----", Mono: true, Invalid: im.err != "", Help: im.err})
		},
		func(gtx C) D {
			return ui.Button(gtx, k, &im.paste, ui.Secondary, icons.Paste, "Paste from clipboard", true)
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &im.pass, ui.FieldOpts{Label: "Passphrase", Hint: "Only if the key has one"})
		},
		func(gtx C) D { return ui.Button(gtx, k, &im.save, ui.Primary, nil, "Import key", true) },
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
		im.h.Notify("Couldn't save the key", err.Error())
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
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: 4, Bottom: 8}.Layout(gtx, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: th.TextMuted, Weight: font.SemiBold}, "PUBLIC KEY")
					})
				}),
				layout.Rigid(func(gtx C) D {
					return ui.Card(gtx, k, func(gtx C) D {
						return layout.UniformInset(14).Layout(gtx, func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Mono: true, MaxLines: 12}, kk.Public)
						})
					})
				}),
			)
		},
		func(gtx C) D { return ui.Button(gtx, k, &d.copy, ui.Primary, icons.Copy, "Copy public key", true) },
		func(gtx C) D { return ui.Button(gtx, k, &d.remove, ui.Destructive, icons.Delete, "Delete key", true) },
	)
}

func (d *Detail) confirmDelete(kk vault.Key) {
	used := 0

	for _, h := range d.h.Vault().Snapshot().Hosts {
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
				d.h.Notify("Couldn't delete the key", err.Error())
				return
			}

			d.h.Pop()
		},
	})
}
