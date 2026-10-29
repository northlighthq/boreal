package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"gioui.org/x/explorer"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/platform/dragdrop"
	"github.com/northlighthq/boreal/internal/platform/shell"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/transfer"
	"github.com/northlighthq/boreal/internal/tunnel"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/toast"
	"github.com/northlighthq/boreal/internal/vault"
)

var _ core.Host = (*App)(nil)

func (a *App) Kit() *kit.Kit                { return a.kit }
func (a *App) Vault() *vault.Vault          { return a.vault }
func (a *App) Toasts() *toast.Stack         { return &a.toasts }
func (a *App) Explorer() *explorer.Explorer { return a.explorer }
func (a *App) Tunnels() *tunnel.Manager     { return a.tunnels }
func (a *App) Settings() config.Settings    { return a.settings }
func (a *App) Profiles() []shell.Profile    { return a.profiles }
func (a *App) CloseDocked(s core.Side)      { a.docked[s] = nil }
func (a *App) ShowTool(t core.Tool)         { a.tools.show(a, t) }
func (a *App) ShowPalette()                 { a.palette.Show() }

func (a *App) ActiveSession() *session.Session {
	p := a.ws.activePane()

	if p == nil {
		return nil
	}

	return p.sess
}

func (a *App) UpdateSettings(s config.Settings) {
	o := a.settings
	themeChanged := s.Theme != o.Theme || s.Accent != o.Accent || s.Density != o.Density || s.Corners != o.Corners || s.TermScheme != o.TermScheme
	a.settings = s
	a.syncLive()

	if themeChanged {
		a.SetTheme(s.Theme)
	}

	a.applySettings()
}

func (a *App) ConnectSplit(h vault.Host) {
	if t := a.ws.current(); t != nil && t.active != nil {
		a.splitWith(a.newSSHPane(h), true)
		return
	}

	a.Connect(h)
}

func (a *App) OpenSFTP(h vault.Host) {
	a.Connect(h)
	a.tools.show(a, a.tools.sftp)
}

func (a *App) QuickConnect(s string) bool {
	h, ok := sshx.ParseQuickConnect(s)

	if !ok {
		return false
	}

	for _, saved := range a.vault.Snapshot().Hosts {
		if saved.Address == h.Address && (h.Username == "" || saved.Username == h.Username) {
			a.Connect(saved)
			return true
		}
	}

	h.Label = h.Address
	a.ws.addTab(a.newSSHPane(h))
	return true
}

func (a *App) RunSnippet(s vault.Snippet, execute bool) {
	p := a.ws.activePane()

	if p == nil {
		a.toasts.Info("No active terminal", "Open a session to run the snippet.")
		return
	}

	cmd := snippetText(s)

	if execute {
		cmd = strings.TrimRight(cmd, "\r") + "\r"
	}

	p.view.SendText(cmd)
	p.view.Focus()
}

func (a *App) splitWith(p *Pane, vertical bool) {
	t := a.ws.current()
	cur := t.active
	leaf, _ := t.root.find(cur, nil)
	*leaf = node{vertical: vertical, ratio: 0.5, a: &node{pane: cur}, b: &node{pane: p}}
	t.active = p
	p.view.Focus()
}

func (a *App) runTyped(cmd string) {
	if target, ok := strings.CutPrefix(cmd, "ssh "); ok && a.QuickConnect(strings.TrimSpace(target)) {
		return
	}

	a.NewLocalTab(a.DefaultProfile())

	if cmd == "" {
		return
	}

	p := a.ws.activePane()

	go func() {
		for range 50 {
			if st, _ := p.sess.State(); st == session.Connected {
				time.Sleep(400 * time.Millisecond)
				p.sess.Send([]byte(cmd + "\r"))
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()
}

var errRuleWithoutHost = errors.New("this rule's host no longer exists")

func (a *App) newTunnels() *tunnel.Manager {
	t := tunnel.New(func(ctx context.Context, r vault.Forward) (*sshx.Client, error) {
		h, ok := a.vault.Host(r.HostID)

		if !ok {
			return nil, errRuleWithoutHost
		}

		keepAlive := time.Duration(a.keepAlive.Load()) * time.Second
		return sshx.Dial(ctx, a.target(h, 0), a.vault, a.hooks(), keepAlive)
	})
	t.OnChange(a.Invalidate)
	t.OnNotify(a.toasts.Info, a.toasts.Success, a.toasts.Error)
	return t
}

func (a *App) dropFiles(paths []string) {
	select {
	case a.drops <- paths:
		a.Invalidate()
	default:
	}
}

func (a *App) handleDrop(paths []string) {
	pane := a.ws.activePane()

	if pane == nil {
		a.toasts.Info("Open a session", "Drop files on an SSH session to upload them via SFTP.")
		return
	}

	if pane.sess.Kind != session.SSH {
		quoted := make([]string, len(paths))

		for i, p := range paths {
			quoted[i] = `"` + p + `"`
		}

		pane.view.SendText(strings.Join(quoted, " "))
		a.Invalidate()
		return
	}

	a.tools.show(a, a.tools.sftp)
	go a.tools.sftp.UploadDropped(pane.sess.ID, paths)
}

func (a *App) Transfers() *transfer.Queue { return a.transfers }

func (a *App) DragOut(paths []string) {
	a.win.Run(func() {
		if err := dragdrop.DragOut(paths); err != nil {
			a.toasts.Error("Couldn't drag the file out", err.Error())
		}
	})
}

func (a *App) OpenURL(link string) { openURL(link) }

func (a *App) syncLive() {
	a.hideAddrs.Store(a.settings.HideAddresses)
	a.keepAlive.Store(int64(a.settings.KeepAlive))
	a.autoUpdate.Store(a.settings.AutoUpdate)
}
