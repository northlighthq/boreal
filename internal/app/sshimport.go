package app

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/northlighthq/boreal/internal/sshconfig"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/vault"
)

func (a *App) ImportSSHConfig() {
	home, _ := os.UserHomeDir()
	entries, err := sshconfig.Parse(filepath.Join(home, ".ssh", "config"))

	if err != nil {
		a.toasts.Error("Couldn't read ~/.ssh/config", err.Error())
		return
	}

	existing := map[string]string{}

	for _, h := range a.vault.Snapshot().Hosts {
		existing[h.Label] = h.ID
	}

	keys := map[string]string{}
	added := 0

	for _, e := range entries {
		if _, ok := existing[e.Alias]; ok {
			continue
		}

		h := vault.Host{
			Label:    e.Alias,
			Address:  e.HostName,
			Username: e.User,
			Port:     e.Port,
			Auth:     vault.AuthAgent,
			Group:    "ssh config",
		}

		if h.Address == "" {
			h.Address = e.Alias
		}

		if e.Identity != "" {
			id, ok := keys[e.Identity]

			if !ok {
				id = a.importKeyFile(e.Identity)
				keys[e.Identity] = id
			}

			if id != "" {
				h.Auth, h.KeyID = vault.AuthKey, id
			}
		}

		saved, err := a.vault.SaveHost(h)

		if err == nil {
			existing[e.Alias] = saved.ID
			added++
		}
	}

	for _, e := range entries {
		if e.Jump == "" {
			continue
		}

		id, ok := existing[e.Alias]
		jid, jok := existing[e.Jump]

		if !ok || !jok {
			continue
		}

		if h, ok := a.vault.Host(id); ok && h.JumpID == "" {
			h.JumpID = jid
			a.vault.SaveHost(h)
		}
	}

	a.toasts.Success("Import complete", strconv.Itoa(added)+" hosts imported from ~/.ssh/config")
}

func (a *App) importKeyFile(path string) string {
	data, err := os.ReadFile(path)

	if err != nil {
		return ""
	}

	k, err := sshx.ImportKey(filepath.Base(path), data, "")

	if err != nil {
		return ""
	}

	saved, err := a.vault.SaveKey(k)

	if err != nil {
		return ""
	}

	return saved.ID
}
