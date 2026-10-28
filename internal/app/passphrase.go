package app

import (
	"fmt"
	"sync"

	"github.com/northlighthq/boreal/internal/app/export"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/vault"
)

type passphrases struct {
	mu sync.Mutex
	m  map[string]string
}

func (p *passphrases) get(id string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.m[id]
	return s, ok
}

func (p *passphrases) set(id, pass string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.m == nil {
		p.m = map[string]string{}
	}

	p.m[id] = pass
}

func (p *passphrases) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.m = nil
}

func (a *App) keyPassphrase(k vault.Key, owner *int) (string, bool) {
	if pass, ok := a.passes.get(k.ID); ok && sshx.CheckPassphrase(k, pass) == nil {
		return pass, true
	}

	msg := fmt.Sprintf("The key “%s” is protected. Enter its passphrase.", k.Label)

	for {
		r := a.ask(&prompt{kind: promptSecret, title: "Key passphrase", message: msg, offer: true, owner: owner})
		pass, remember, ok := r.text, r.remember, r.ok

		if !ok {
			return "", false
		}

		if sshx.CheckPassphrase(k, pass) != nil {
			msg = fmt.Sprintf("That isn't the passphrase of “%s”. Try again.", k.Label)
			continue
		}

		if remember {
			k.Passphrase = pass
			a.vault.SaveKey(k)
		} else {
			a.passes.set(k.ID, pass)
		}

		return pass, true
	}
}

func (a *App) AskKeyPassphrase(message string) (string, bool, bool) {
	r := a.ask(&prompt{kind: promptSecret, title: "Key passphrase", message: message, offer: true})
	return r.text, r.remember, r.ok
}

func (a *App) RememberPassphrase(keyID, pass string) {
	a.passes.set(keyID, pass)
}

func (a *App) AskPassword(title, message string) (string, bool) {
	r := a.ask(&prompt{kind: promptSecret, title: title, message: message})
	return r.text, r.ok
}

func (a *App) ExportData()      { a.Open(export.New(a)) }
func (a *App) ImportData()      { go export.Import(a) }
func (a *App) ExportSSHConfig() { go export.SSHConfig(a) }
