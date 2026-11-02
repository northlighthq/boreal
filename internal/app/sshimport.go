package app

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/northlighthq/boreal/internal/sshconfig"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	sshImportFolder = "ssh config"
	maxSkippedShown = 6
)

type sshImport struct {
	a       *App
	byLabel map[string]string
	keys    map[string]string
	skipped map[string]int
	added   int
}

func (a *App) ImportSSHConfig() {
	home, _ := os.UserHomeDir()
	res, err := sshconfig.Parse(filepath.Join(home, ".ssh", "config"))

	if err != nil {
		a.toasts.Error("Couldn't read ~/.ssh/config", err.Error())
		return
	}

	imp := &sshImport{a: a, byLabel: map[string]string{}, keys: map[string]string{}, skipped: map[string]int{}}

	for _, s := range res.Skipped {
		imp.skipped[s.Name] += s.Count
	}

	for _, h := range a.vault.Snapshot().Hosts {
		imp.byLabel[h.Label] = h.ID
	}

	for _, e := range res.Entries {
		imp.host(e)
	}

	for _, e := range res.Entries {
		imp.chain(e)
	}

	a.toasts.Success("Import complete", imp.summary())
}

func (imp *sshImport) host(e sshconfig.Entry) {
	if _, ok := imp.byLabel[e.Alias]; ok {
		return
	}

	h := vault.Host{
		Label:          e.Alias,
		Address:        e.HostName,
		Username:       e.User,
		Port:           e.Port,
		Auth:           vault.AuthAgent,
		Group:          sshImportFolder,
		ForwardAgent:   e.ForwardAgent,
		IdentitiesOnly: e.IdentitiesOnly,
	}

	if len(e.Jump) == 0 {
		h.ProxyCommand = e.ProxyCommand
	}

	if h.Address == "" {
		h.Address = e.Alias
	}

	if id := imp.key(e.Identity, e.Certificate); id != "" {
		h.Auth, h.KeyID = vault.AuthKey, id
	}

	saved, err := imp.a.vault.SaveHost(h)

	if err != nil {
		return
	}

	imp.byLabel[e.Alias] = saved.ID
	imp.added++
}

func (imp *sshImport) key(path, certPath string) string {
	if path == "" {
		return ""
	}

	ref := path + "\x00" + certPath

	if id, ok := imp.keys[ref]; ok {
		return id
	}

	id := imp.importKey(path, certPath)
	imp.keys[ref] = id
	return id
}

func (imp *sshImport) importKey(path, certPath string) string {
	data, err := os.ReadFile(path)

	if err != nil {
		imp.skipped["IdentityFile (unreadable)"]++
		return ""
	}

	k, err := sshx.ImportKey(filepath.Base(path), data, "")

	if err != nil {
		imp.skipped["IdentityFile (unsupported key)"]++
		return ""
	}

	if certPath == "" && fileExists(path+"-cert.pub") {
		certPath = path + "-cert.pub"
	}

	if certPath != "" {
		imp.certificate(&k, certPath)
	}

	saved, err := imp.a.vault.SaveKey(k)

	if err != nil {
		return ""
	}

	return saved.ID
}

func (imp *sshImport) certificate(k *vault.Key, path string) {
	raw, err := os.ReadFile(path)

	if err != nil {
		imp.skipped["CertificateFile (unreadable)"]++
		return
	}

	text := strings.TrimSpace(string(raw))

	if err := sshx.CheckCertificate(*k, text); err != nil {
		imp.skipped["CertificateFile (doesn't match the key)"]++
		return
	}

	k.Certificate = text
}

func (imp *sshImport) chain(e sshconfig.Entry) {
	if len(e.Jump) == 0 {
		return
	}

	next, ok := imp.byLabel[e.Alias]

	if !ok {
		return
	}

	for i := len(e.Jump) - 1; i >= 0; i-- {
		hop := imp.hop(e.Jump[i])

		if hop == "" || hop == next {
			imp.skipped["ProxyJump "+e.Jump[i]]++
			return
		}

		if !imp.link(next, hop) {
			return
		}

		next = hop
	}
}

func (imp *sshImport) link(id, jumpID string) bool {
	h, ok := imp.a.vault.Host(id)

	if !ok {
		return false
	}

	if h.JumpID == jumpID {
		return true
	}

	if h.JumpID != "" {
		imp.skipped["ProxyJump (host already uses another jump host)"]++
		return false
	}

	h.JumpID = jumpID
	h.ProxyCommand = ""
	_, err := imp.a.vault.SaveHost(h)
	return err == nil
}

func (imp *sshImport) hop(spec string) string {
	if id, ok := imp.byLabel[spec]; ok {
		return id
	}

	user, addr, hasUser := strings.Cut(spec, "@")

	if !hasUser {
		user, addr = "", spec
	}

	host, portText, err := net.SplitHostPort(addr)
	port := 22

	if err == nil {
		port, err = strconv.Atoi(portText)
	}

	if err != nil {
		host, port = strings.Trim(addr, "[]"), 22
	}

	if id, ok := imp.byLabel[host]; ok && user == "" {
		return id
	}

	if host == "" {
		return ""
	}

	saved, err := imp.a.vault.SaveHost(vault.Host{
		Label:    spec,
		Address:  host,
		Port:     port,
		Username: user,
		Auth:     vault.AuthAgent,
		Group:    sshImportFolder,
	})

	if err != nil {
		return ""
	}

	imp.byLabel[spec] = saved.ID
	imp.added++
	return saved.ID
}

func (imp *sshImport) summary() string {
	msg := fmt.Sprintf("%d hosts imported from ~/.ssh/config.", imp.added)

	if len(imp.skipped) == 0 {
		return msg
	}

	names := make([]string, 0, len(imp.skipped))

	for name, n := range imp.skipped {
		if n > 1 {
			name += " ×" + strconv.Itoa(n)
		}

		names = append(names, name)
	}

	slices.Sort(names)

	if len(names) > maxSkippedShown {
		names = append(names[:maxSkippedShown], fmt.Sprintf("%d more", len(names)-maxSkippedShown))
	}

	return msg + " Not imported: " + strings.Join(names, ", ") + "."
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
