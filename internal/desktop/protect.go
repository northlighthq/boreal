package desktop

import (
	"fmt"

	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/desktop/dialogs"
)

const protectHint = "Boreal can't reach a system keyring on this computer, so it can't keep your hosts and keys safe on its own. Choose a master password: Boreal asks for it every time it opens."

func (a *App) requireProtection() {
	if a.protecting != nil || !a.vault.NeedsPassword() {
		return
	}

	pass := dialogs.NewField("Master password", fmt.Sprintf("At least %d characters", vault.MinPassword), "")
	pass.Secret = true
	again := dialogs.NewField("Repeat it", "", "")
	again.Secret = true

	f := dialogs.NewForm(a.kit, "Protect your vault", "Set password", pass, again)
	f.Subtitle = protectHint
	f.OnOK = func(values []string) bool {
		if err := vault.CheckPassword(values[0], values[1]); err != nil {
			f.Subtitle = err.Error()
			return false
		}

		if err := a.vault.SetPassword("", values[0]); err != nil {
			f.Subtitle = err.Error()
			return false
		}

		a.protecting = nil
		a.toasts.Success("Vault protected", "Boreal will ask for this password when it opens.")
		return true
	}

	a.protecting = f
	a.Open(f)
}
