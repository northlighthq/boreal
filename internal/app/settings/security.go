package settings

import (
	"errors"
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const minMasterPassword = 10

var lockAfterMinutes = []int{0, 5, 15, 30, 60}

var lockAfterLabels = []string{"Never", "After 5 minutes", "After 15 minutes", "After 30 minutes", "After 1 hour"}

type security struct {
	current, next, confirm widget.Editor
	setPassword, remove    widget.Clickable
	lockNow, forget        widget.Clickable
	osUnlock               widget.Bool
	lockAfter              kit.Select
	message                string
	failed                 bool

	export, imp, sshConfig widget.Clickable
}

func (s *security) init(lockAfter int) {
	for _, e := range []*widget.Editor{&s.current, &s.next, &s.confirm} {
		e.SingleLine, e.Mask = true, '•'
	}

	for i, m := range lockAfterMinutes {
		if m == lockAfter {
			s.lockAfter.Value = i
		}
	}
}

func (s *security) report(err error, ok string) {
	s.failed = err != nil
	s.message = ok

	switch {
	case errors.Is(err, vault.ErrWrongPassword):
		s.message = "The current password is wrong."
	case err != nil:
		s.message = err.Error()
	}
}

func (s *security) update(a core.Host, gtx C) {
	v := a.Vault()

	if s.osUnlock.Update(gtx) {
		s.report(v.SetOSUnlock(s.osUnlock.Value), "")
	}

	s.osUnlock.Value = v.OSUnlock()

	if s.setPassword.Clicked(gtx) {
		switch next := s.next.Text(); {
		case len([]rune(next)) < minMasterPassword:
			s.report(fmt.Errorf("the master password must be at least %d characters long", minMasterPassword), "")
		case next != s.confirm.Text():
			s.report(errors.New("the new passwords don't match"), "")
		default:
			s.report(v.SetPassword(s.current.Text(), next), "Master password saved.")
		}

		if !s.failed {
			s.current.SetText("")
			s.next.SetText("")
			s.confirm.SetText("")
		}
	}

	if s.remove.Clicked(gtx) {
		s.report(v.SetPassword(s.current.Text(), ""), "Master password removed. The vault opens with your "+vault.OSOwner+".")

		if !s.failed {
			s.current.SetText("")
		}
	}

	if s.forget.Clicked(gtx) {
		s.report(v.ForgetPassphrases(), "Saved passphrases forgotten. Boreal will ask for them when connecting.")
	}

	if s.lockNow.Clicked(gtx) {
		a.LockVault()
	}

	if s.export.Clicked(gtx) {
		a.ExportData()
	}

	if s.imp.Clicked(gtx) {
		a.ImportData()
	}

	if s.sshConfig.Clicked(gtx) {
		a.ExportSSHConfig()
	}
}

func (d *Dialog) securityPage(a core.Host, gtx C, row func(string, layout.Widget) layout.FlexChild, left func(layout.Widget) layout.Widget) []layout.FlexChild {
	k := a.Kit()
	th := k.Th
	s := &d.sec
	v := a.Vault()
	has := v.HasPassword()

	muted := func(text string) layout.Widget {
		return func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, text)
		}
	}

	field := func(ed *widget.Editor, hint string) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(320))
			return k.Field(gtx, ed, kit.FieldOpts{Icon: icons.Lock, Hint: hint})
		}
	}

	status := "Not set. The vault opens with your " + vault.OSOwner + " only."

	if has {
		status = "Set."
	}

	children := []layout.FlexChild{
		row("Storage", muted("Encrypted with XChaCha20-Poly1305 in %AppData%\\Boreal."+vaultSummary(a))),
		row("Master password", muted(status)),
	}

	if has {
		children = append(children, row("Current password", field(&s.current, "Current master password")))
	}

	label := "Set password"

	if has {
		label = "Change password"
	}

	children = append(children,
		row("New password", field(&s.next, fmt.Sprintf("At least %d characters", minMasterPassword))),
		row("Repeat it", field(&s.confirm, "New password again")),
		row("", left(func(gtx C) D {
			buttons := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return k.Button(gtx, &s.setPassword, kit.Secondary, icons.Key, label) }),
			}

			if has {
				buttons = append(buttons,
					layout.Rigid(kit.Gap(8)),
					layout.Rigid(func(gtx C) D { return k.Button(gtx, &s.remove, kit.Danger, nil, "Remove") }),
				)
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
		})),
	)

	if s.message != "" {
		color := th.TextMuted

		if s.failed {
			color = th.Danger
		}

		children = append(children, row("", func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: color, MaxLines: 2}, s.message)
		}))
	}

	if has {
		children = append(children,
			row("Unlock with "+vault.OSName, left(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return k.Toggle(gtx, &s.osUnlock) }),
					layout.Rigid(kit.Gap(12)),
					layout.Rigid(muted("Open the vault on this PC without typing the password")),
				)
			})),
			row("Lock automatically", func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(220))
				return k.Select(gtx, &s.lockAfter, lockAfterLabels)
			}),
			row("", left(func(gtx C) D {
				return k.Button(gtx, &s.lockNow, kit.Secondary, icons.Lock, "Lock now  ·  Ctrl+Shift+L")
			})),
		)
	}

	if n := v.SavedPassphrases(); n > 0 {
		children = append(children, row("Key passphrases", left(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(muted(fmt.Sprintf("%d keys keep their passphrase in the vault.", n))),
				layout.Rigid(kit.Gap(12)),
				layout.Rigid(func(gtx C) D { return k.Button(gtx, &s.forget, kit.Secondary, nil, "Forget them") }),
			)
		})))
	}

	return children
}

func (d *Dialog) dataPage(a core.Host, row func(string, layout.Widget) layout.FlexChild) []layout.FlexChild {
	k := a.Kit()
	th := k.Th
	s := &d.sec

	item := func(clk *widget.Clickable, ic *icons.Icon, title, text string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 22}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Weight: font.Medium}, title) }),
							layout.Rigid(kit.Gap(4)),
							layout.Rigid(func(gtx C) D {
								return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, text)
							}),
						)
					}),
					layout.Rigid(kit.Gap(24)),
					layout.Rigid(func(gtx C) D { return k.Button(gtx, clk, kit.Secondary, ic, title+"…") }),
				)
			})
		})
	}

	return []layout.FlexChild{
		item(&s.export, icons.Upload, "Export", "Save hosts, keys, snippets and more to a .boreal file encrypted with a password of your choice. Passwords are left out unless you include them."),
		item(&s.imp, icons.Download, "Import", "Add the contents of a .boreal or .platty file to this vault. Nothing is deleted, and host keys you already trust are kept."),
		item(&s.sshConfig, icons.File, "SSH config", "Write your hosts in ~/.ssh/config format, without passwords or keys, to use them with other tools."),
	}
}
