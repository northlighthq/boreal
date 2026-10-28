package keychain

import (
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	keyMenuCopy = iota
	keyMenuRename
	keyMenuCert
	keyMenuSep
	keyMenuDelete
)

type Panel struct {
	h core.Host

	generate widget.Clickable
	imp      widget.Clickable
	emptyGen widget.Clickable
	emptyImp widget.Clickable
	list     kit.ScrollList
	menu     kit.Menu
	rows     map[string]*kit.Row
	keys     []vault.Key
	selected string
	menuKey  vault.Key

	renaming string
	rename   kit.InlineEdit
	copyBtn  map[string]*widget.Clickable
	edit     map[string]*widget.Clickable
}

func (p *Panel) startRename(a core.Host, key vault.Key) {
	p.renaming = key.ID
	p.rename.Start(a.Kit(), key.Label)
}

func (p *Panel) commitRename(a core.Host, key vault.Key) {
	p.renaming = ""

	if name := strings.TrimSpace(p.rename.Text()); name != "" && name != key.Label {
		key.Label = name
		a.Vault().SaveKey(key)
	}
}

func (p *Panel) Title() string     { return "Keychain" }
func (p *Panel) Icon() *icons.Icon { return icons.KeyOutline }
func (p *Panel) Side() core.Side   { return core.Right }
func (p *Panel) Shortcut() string  { return "Alt+2" }

func (p *Panel) Refresh() {
	a := p.h
	p.keys = a.Vault().Snapshot().Keys
}

func (p *Panel) Actions(gtx C) D {
	a := p.h
	k := a.Kit()
	th := a.Kit().Th

	if p.generate.Clicked(gtx) {
		a.OpenIn(p, NewGenerateKey(a))
	}

	if p.imp.Clicked(gtx) {
		go importKey(a)
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return k.IconButtonSized(gtx, &p.generate, icons.Add, false, 26, 18, th.TextMuted) }),
		layout.Rigid(func(gtx C) D { return k.IconButtonSized(gtx, &p.imp, icons.Download, false, 26, 18, th.TextMuted) }),
	)
}

func (p *Panel) Body(gtx C) D {
	a := p.h
	k := a.Kit()
	th := a.Kit().Th

	if p.rows == nil {
		p.rows = map[string]*kit.Row{}
	}

	p.handleMenu(a, gtx)

	if len(p.keys) == 0 {
		if p.emptyGen.Clicked(gtx) {
			a.OpenIn(p, NewGenerateKey(a))
		}

		if p.emptyImp.Clicked(gtx) {
			go importKey(a)
		}

		return a.Kit().EmptyState(gtx, icons.Key, "Keychain is empty",
			"Generate an ed25519 key or import an existing one. Keys are encrypted with your "+vault.OSOwner+".",
			func(gtx C) D { return k.Button(gtx, &p.emptyGen, kit.Primary, icons.Add, "Generate key") },
			func(gtx C) D { return k.Button(gtx, &p.emptyImp, kit.Secondary, icons.Download, "Import…") },
		)
	}

	dims := k.List(gtx, &p.list, len(p.keys), func(gtx C, i int) D {
		key := p.keys[i]
		r := p.rows[key.ID]

		if r == nil {
			r = &kit.Row{}
			p.rows[key.ID] = r
		}

		ev := r.Update(gtx)

		if ev.Click {
			p.selected = key.ID
		}

		if ev.DoubleClick {
			p.startRename(a, key)
		}

		if p.copyBtn == nil {
			p.copyBtn, p.edit = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
		}

		cp, ed := p.copyBtn[key.ID], p.edit[key.ID]

		if cp == nil {
			cp, ed = &widget.Clickable{}, &widget.Clickable{}
			p.copyBtn[key.ID], p.edit[key.ID] = cp, ed
		}

		if cp.Clicked(gtx) {
			p.copyPublic(a, gtx, key)
		}

		if ed.Clicked(gtx) {
			p.startRename(a, key)
		}

		if ev.Context {
			p.selected = key.ID
			p.menuKey = key
			p.menu.Open([]kit.MenuItem{
				keyMenuCopy:   {Label: "Copy public key", Icon: icons.Copy},
				keyMenuRename: {Label: "Rename…", Icon: icons.Edit},
				keyMenuCert:   certItem(key),
				keyMenuSep:    kit.Separator(),
				keyMenuDelete: {Label: "Delete", Icon: icons.Delete, Danger: true},
			}, ev.At.Add(image.Pt(0, gtx.Dp(4))))
		}

		return k.Row(gtx, r, kit.RowOpts{Selected: p.selected == key.ID, Focused: true, Height: 62}, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icons.Key.Layout(gtx, 18, th.TextMuted) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if p.renaming == key.ID {
								dims, res := p.rename.Layout(gtx, a.Kit(), "Key name")

								switch res {
								case kit.EditCommit:
									p.commitRename(a, key)
								case kit.EditCancel:
									p.renaming = ""
								}

								return dims
							}

							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									gtx.Constraints.Max.X = max(gtx.Constraints.Max.X-gtx.Dp(78), 0)
									return k.Label(gtx, kit.TextStyle{Weight: font.Medium, MaxLines: 1}, key.Label)
								}),
								layout.Rigid(kit.Gap(6)),
								layout.Rigid(func(gtx C) D {
									return k.Badge(gtx, strings.ToUpper(strings.TrimPrefix(key.Type, "ssh-")), th.TextMuted)
								}),
								layout.Rigid(func(gtx C) D {
									if key.Certificate == "" {
										return D{}
									}

									return layout.Inset{Left: 6}.Layout(gtx, func(gtx C) D { return k.Badge(gtx, "CERT", th.TextMuted) })
								}),
							)
						}),
						layout.Rigid(kit.Gap(4)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: th.TextFaint, Mono: true}, sshx.Fingerprint(key.Public))
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if p.renaming == key.ID || !(r.Hovered() || cp.Hovered() || ed.Hovered()) {
						return D{}
					}

					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return k.IconButtonSized(gtx, ed, icons.Edit, false, 26, 15, th.TextMuted) }),
						layout.Rigid(func(gtx C) D { return k.IconButtonSized(gtx, cp, icons.Copy, false, 26, 15, th.TextMuted) }),
					)
				}),
			)
		})
	})

	k.Menu(gtx, &p.menu)
	return dims
}

func (p *Panel) copyPublic(a core.Host, gtx C, key vault.Key) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(key.Public))})
	a.Toasts().Info("Public key copied", "Paste it into ~/.ssh/authorized_keys on the server.")
}

func (p *Panel) handleMenu(a core.Host, gtx C) {
	i, ok := p.menu.Chosen()

	if !ok {
		return
	}

	key := p.menuKey

	switch i {
	case keyMenuCopy:
		p.copyPublic(a, gtx, key)
	case keyMenuRename:
		p.startRename(a, key)
	case keyMenuCert:
		if key.Certificate != "" {
			key.Certificate = ""
			a.Vault().SaveKey(key)
			a.Toasts().Info("Certificate removed", key.Label)
			return
		}

		go attachCertificate(a, key)
	case keyMenuDelete:
		a.Open(dialogs.NewConfirm(a.Kit(), "Delete this key?", "“"+key.Label+"” will be removed from the vault. Hosts that use it will sign in with the agent instead.", "Delete", func() {
			a.Vault().DeleteKey(key.ID)
		}))
	}
}

type GenerateKey struct {
	*dialogs.Form
	h         core.Host
	keep      widget.Bool
	generated *vault.Key
	copy      widget.Clickable
	done      widget.Clickable
	dlg2      kit.Dialog
}

func NewGenerateKey(a core.Host) *GenerateKey {
	pass := dialogs.NewField("Passphrase", "Optional, protects the private key", "")
	pass.Secret = true
	g := &GenerateKey{h: a, Form: dialogs.NewForm(a.Kit(), "New SSH key", "Generate key", dialogs.NewField("Name", "e.g. my-laptop", ""), pass)}
	g.Subtitle = "Creates an ed25519 key pair, stored encrypted in your vault."
	g.Extra = func(gtx C) D {
		return a.Kit().Checkbox(gtx, &g.keep, "Remember the passphrase in the vault")
	}

	g.OnOK = func(v []string) bool {
		label := strings.TrimSpace(v[0])

		if label == "" {
			label = "platty"
		}

		key, err := sshx.GenerateEd25519(label, v[1])

		if err != nil {
			a.Toasts().Error("Couldn't generate the key", err.Error())
			return true
		}

		if !g.keep.Value {
			key.Passphrase = ""
		}

		saved, err := a.Vault().SaveKey(key)

		if err != nil {
			a.Toasts().Error("Couldn't save the key", err.Error())
			return true
		}

		if v[1] != "" && !g.keep.Value {
			a.RememberPassphrase(saved.ID, v[1])
		}

		g.generated = &saved
		return false
	}
	return g
}

func (g *GenerateKey) Layout(gtx C) bool {
	a := g.h

	if g.generated == nil {
		return g.Form.Layout(gtx)
	}

	g.dlg2.Docked = g.Form.Dialog().Docked

	k := a.Kit()

	if g.copy.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(g.generated.Public))})
		a.Toasts().Info("Public key copied", "")
	}

	if g.done.Clicked(gtx) {
		return true
	}

	return k.Dialog(gtx, &g.dlg2, kit.DialogOpts{
		Title:    "Your key is ready",
		Subtitle: "Add this public key to ~/.ssh/authorized_keys on each server you want to reach.",
		Width:    560,
		Body:     func(gtx C) D { return keyBox(a, gtx, *g.generated) },
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &g.copy, kit.Secondary, icons.Copy, "Copy") },
			func(gtx C) D { return k.Button(gtx, &g.done, kit.Primary, nil, "Done") },
		),
	})
}

func importKey(a core.Host) {
	f, err := a.Explorer().ChooseFile()

	if err != nil {
		return
	}

	defer f.Close()

	data, err := io.ReadAll(f)

	if err != nil {
		a.Toasts().Error("Couldn't read the file", err.Error())
		return
	}

	label := "key"

	if named, ok := f.(*os.File); ok {
		label = filepath.Base(named.Name())
	}

	key, err := sshx.ImportKey(label, data, "")
	pass, remember := "", false

	if sshx.IsPassphraseMissing(err) {
		msg := "The key “" + label + "” is encrypted. Enter its passphrase."

		for {
			var ok bool
			pass, remember, ok = a.AskKeyPassphrase(msg)

			if !ok {
				return
			}

			if key, err = sshx.ImportKey(label, data, pass); err == nil {
				break
			}

			msg = "That isn't the passphrase of “" + label + "”. Try again."
		}
	}

	if !remember {
		key.Passphrase = ""
	}

	if named, ok := f.(*os.File); ok {
		if cert, err := os.ReadFile(named.Name() + "-cert.pub"); err == nil && sshx.CheckCertificate(key, string(cert)) == nil {
			key.Certificate = string(cert)
		}
	}

	if err != nil {
		a.Toasts().Error("Invalid key", err.Error())
		return
	}

	saved, err := a.Vault().SaveKey(key)

	if err != nil {
		a.Toasts().Error("Couldn't save the key", err.Error())
		return
	}

	if pass != "" && !remember {
		a.RememberPassphrase(saved.ID, pass)
	}

	a.Toasts().Success("Key imported", label)
}

func keyBox(a core.Host, gtx C, key vault.Key) D {
	k := a.Kit()
	th := a.Kit().Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(12).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icons.Key.Layout(gtx, 16, th.TextMuted) }),
					layout.Rigid(kit.Gap(8)),
					layout.Flexed(1, func(gtx C) D { return k.Label(gtx, kit.TextStyle{Weight: font.Medium}, key.Label) }),
					layout.Rigid(func(gtx C) D {
						return k.Badge(gtx, strings.ToUpper(strings.TrimPrefix(key.Type, "ssh-")), th.TextMuted)
					}),
				)
			}),
			layout.Rigid(kit.Gap(4)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint, Mono: true}, sshx.Fingerprint(key.Public))
			}),
			layout.Rigid(kit.Gap(10)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Mono: true, Size: th.TextSizeSmall, MaxLines: 8, Color: th.TextMuted}, key.Public)
			}),
		)
	})
	call := rec.Stop()
	r := image.Rectangle{Max: dims.Size}
	kit.RRect(gtx, r, gtx.Dp(th.Radius), kit.Lerp(th.Panel, th.Window, 0.6))
	kit.Border(gtx, r, gtx.Dp(th.Radius), 1, th.Border)
	call.Add(gtx.Ops)
	return dims
}

func New(h core.Host) *Panel {
	return &Panel{h: h}
}

type (
	C = core.C
	D = core.D
)

func (p *Panel) MenuOpen() bool {
	return p.menu.IsOpen()
}

func (p *Panel) Renaming() string {
	return p.renaming
}

func (p *Panel) RenameEditor() *widget.Editor {
	return p.rename.Editor()
}

func certItem(key vault.Key) kit.MenuItem {
	if key.Certificate != "" {
		return kit.MenuItem{Label: "Remove certificate", Icon: icons.Shield}
	}

	return kit.MenuItem{Label: "Attach certificate…", Icon: icons.Shield}
}

func attachCertificate(a core.Host, key vault.Key) {
	f, err := a.Explorer().ChooseFile(".pub")

	if err != nil {
		return
	}

	data, err := io.ReadAll(f)
	f.Close()

	if err != nil {
		a.Toasts().Error("Couldn't read the file", err.Error())
		return
	}

	if err := sshx.CheckCertificate(key, string(data)); err != nil {
		a.Toasts().Error("Invalid certificate", err.Error())
		return
	}

	key.Certificate = string(data)

	if _, err := a.Vault().SaveKey(key); err != nil {
		a.Toasts().Error("Couldn't save the certificate", err.Error())
		return
	}

	a.Toasts().Success("Certificate attached", key.Label)
}
