package settings

import (
	"errors"
	"fmt"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/keys"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/desktop/base"
	"github.com/northlighthq/boreal/internal/platform/hello"
	"github.com/northlighthq/boreal/internal/platform/sessionlock"
)

const minMasterPassword = vault.MinPassword

var lockAfterMinutes = []int{0, 1, 5, 15, 30, 60, 120, 240}

var lockAfterLabels = []string{"Never", "After 1 minute", "After 5 minutes", "After 15 minutes", "After 30 minutes", "After 1 hour", "After 2 hours", "After 4 hours"}

type security struct {
	current, next, confirm widget.Editor
	setPassword, remove    widget.Clickable
	lockNow, forget        widget.Clickable
	osUnlock               widget.Bool
	osUnlockJob            async
	readKnown, writeKnown  widget.Bool
	lockOnSystem, hello    widget.Bool
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

func (s *security) update(a base.Host, gtx C) {
	v := a.Vault()

	if err, ok := s.osUnlockJob.poll(); ok {
		s.report(err, "")
	}

	if s.osUnlock.Update(gtx) && !s.osUnlockJob.busy() {
		on := s.osUnlock.Value
		s.osUnlockJob.run(a, func() error { return v.SetOSUnlock(on) })
	}

	if !s.osUnlockJob.busy() {
		s.osUnlock.Value = v.OSUnlock()
	}

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

func (d *Dialog) securityPage(a base.Host, p page) []layout.FlexChild {
	k := p.k
	th := k.Th
	s := &d.sec
	v := a.Vault()
	has := v.HasPassword()
	helloRow := a.HelloAvailable() && v.OSUnlock()
	canLock := has || helloRow && s.hello.Value

	var lock []layout.Widget

	if helloRow {
		lock = append(lock, p.item(hello.Name, "Ask for "+hello.Name+" to unlock Boreal after it locks.", p.toggle(&s.hello)))

		if !has {
			lock = append(lock, p.note(hello.Name+" locks Boreal's window, but programs running as you could still read the vault. Set a master password to encrypt it with something only you know.", th.Warning))
		}
	}

	switch {
	case canLock:
		lock = append(lock, p.item("Lock automatically", "After this long without using Boreal.", p.choose(&s.lockAfter, lockAfterLabels)))

		if sessionlock.Supported {
			lock = append(lock, p.item("Lock with "+vault.System, "Also lock when "+vault.System+" locks the session.", p.toggle(&s.lockOnSystem)))
		}

		lock = append(lock, p.item("Lock now", keys.Label("Ctrl+Shift+L"), func(gtx C) D { return k.Button(gtx, &s.lockNow, kit.Secondary, icons.Lock, "Lock") }))
	case helloRow:
		lock = append(lock, p.note("Boreal stays unlocked. Turn on "+hello.Name+" or set a master password to lock it.", th.TextMuted))
	default:
		lock = append(lock, p.note("Boreal stays unlocked. Set a master password to lock it.", th.TextMuted))
	}

	status := "Not set. The vault opens with your " + vault.OSOwner + " only."

	if has {
		status = "Set. Boreal asks for it when it opens."
	}

	vaultRows := []layout.Widget{
		p.item("Storage", "Encrypted with XChaCha20-Poly1305"+storageDir()+". "+vaultSummary(a), nil),
		p.item("Master password", status, nil),
	}

	if has {
		vaultRows = append(vaultRows, p.item("Unlock with "+vault.OSName, "Open the vault on this "+vault.Device+" without typing the password.", p.toggle(&s.osUnlock)))
	}

	title, label := "Set a master password", "Set password"

	if has {
		title, label = "Change the master password", "Change password"
	}

	pass := func(ed *widget.Editor, hint string) layout.Widget {
		return p.width(wideField, func(gtx C) D { return k.Field(gtx, ed, kit.FieldOpts{Icon: icons.Lock, Hint: hint}) })
	}

	var change []layout.Widget

	if has {
		change = append(change, p.item("Current password", "", pass(&s.current, "Current master password")))
	}

	change = append(change,
		p.item("New password", fmt.Sprintf("At least %d characters.", minMasterPassword), pass(&s.next, "New password")),
		p.item("Repeat it", "", pass(&s.confirm, "New password again")),
		p.block(func(gtx C) D {
			buttons := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return k.Button(gtx, &s.setPassword, kit.Secondary, icons.Key, label) }),
			}

			if has {
				buttons = append(buttons,
					layout.Rigid(kit.Gap(8)),
					layout.Rigid(func(gtx C) D { return k.Button(gtx, &s.remove, kit.Danger, nil, "Remove password") }),
				)
			}

			if s.message != "" {
				c := th.TextMuted

				if s.failed {
					c = th.Danger
				}

				buttons = append(buttons,
					layout.Rigid(kit.Gap(14)),
					layout.Flexed(1, func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: c, MaxLines: 2}, s.message)
					}),
				)
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
		}),
	)

	children := []layout.FlexChild{
		p.header(settingsPages[pageSecurity].title),
		p.group("Lock", lock...),
		p.group("Vault", vaultRows...),
		p.group(title, change...),
	}

	if n := v.SavedPassphrases(); n > 0 {
		children = append(children, p.group("Key passphrases",
			p.item(fmt.Sprintf("%d keys keep their passphrase in the vault", n), "Forget them to be asked each time you connect.", func(gtx C) D {
				return k.Button(gtx, &s.forget, kit.Secondary, nil, "Forget them")
			}),
		))
	}

	return children
}

func (d *Dialog) dataPage(p page) []layout.FlexChild {
	k := p.k
	s := &d.sec

	action := func(clk *widget.Clickable, ic *icons.Icon, label string) layout.Widget {
		return func(gtx C) D { return k.Button(gtx, clk, kit.Secondary, ic, label) }
	}

	return []layout.FlexChild{
		p.header(settingsPages[pageData].title),
		p.group("Backup",
			p.item("Export", "Save hosts, keys, snippets and more to a .boreal file encrypted with a password of your choice. Passwords are left out unless you include them.", action(&s.export, icons.Upload, "Export…")),
			p.item("Import", "Add the contents of a .boreal or .platty file to this vault. Nothing is deleted, and host keys you already trust are kept.", action(&s.imp, icons.Download, "Import…")),
		),
		p.group("Other tools",
			p.item("SSH config", "Write your hosts in ~/.ssh/config format, without passwords or keys, to use them with other tools.", action(&s.sshConfig, icons.File, "Export…")),
		),
	}
}

var storageDir = sync.OnceValue(func() string {
	dir, err := config.Dir()

	if err != nil {
		return ""
	}

	return " in " + dir
})
