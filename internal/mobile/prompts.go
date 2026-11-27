package mobile

import (
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/termview"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

const pastePreview = 4

type reply struct {
	ok   bool
	text string
}

func (a *App) hooks() sshx.Hooks {
	return sshx.Hooks{
		ConfirmHostKey: a.confirmHostKey,
		AskPassword: func(prompt string) (string, bool) {
			return a.askSecret("Password", prompt, "Password")
		},
		KeyPassphrase: func(k vault.Key) (string, bool) {
			return a.askSecret("Key passphrase", "Enter the passphrase for "+k.Label+".", "Passphrase")
		},
	}
}

func (a *App) confirmHostKey(info sshx.HostKeyInfo) bool {
	d := &ui.Dialog{
		Title:   "Trust " + info.Host + "?",
		Message: "This is the first time you connect to this server. Check that its fingerprint matches before you continue.",
		Detail:  info.KeyType + "\n" + info.Fingerprint,
		Confirm: "Trust and connect",
		Cancel:  "Cancel",
	}

	if info.Changed {
		d.Title = "The server key changed"
		d.Message = "The key of " + info.Host + " isn't the one Boreal saved. Someone could be intercepting the connection. Only continue if you know the server was reinstalled."
		d.Detail = "Before: " + info.OldFingerprint + "\nNow: " + info.Fingerprint
		d.Confirm = "Trust the new key"
		d.Danger = true
	}

	return a.wait(d).ok
}

func (a *App) askSecret(title, message, hint string) (string, bool) {
	ed := new(widget.Editor)
	ui.SetupField(ed, key.HintPassword, true)

	r := a.wait(&ui.Dialog{
		Title:   title,
		Message: message,
		Field:   ed,
		Hint:    hint,
		Confirm: "Continue",
		Cancel:  "Cancel",
	})

	return r.text, r.ok
}

func (a *App) wait(d *ui.Dialog) reply {
	ch := make(chan reply, 1)
	d.Reply = func(ok bool, text string) { ch <- reply{ok: ok, text: text} }
	a.Ask(d)
	return <-ch
}

func (a *App) guardPaste(view *termview.View, title, text string) bool {
	trimmed := strings.TrimRight(text, "\r\n")
	lines := strings.Count(trimmed, "\n") + 1

	if lines < 2 || view.BracketedPaste() {
		return false
	}

	preview := strings.Split(trimmed, "\n")

	if len(preview) > pastePreview {
		preview = append(preview[:pastePreview], "…")
	}

	a.Ask(&ui.Dialog{
		Title:   "Paste " + strconv.Itoa(lines) + " lines?",
		Message: "Each line runs on " + title + " as soon as it arrives.",
		Detail:  strings.Join(preview, "\n"),
		Confirm: "Paste",
		Cancel:  "Cancel",
		Reply: func(ok bool, _ string) {
			if ok {
				view.Paste(text)
			}
		},
	})

	return true
}
