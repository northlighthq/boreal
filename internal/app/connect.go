package app

import (
	"fmt"
	"sync"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/platform/shell"
	"github.com/northlighthq/boreal/internal/privacy"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/termview"
	"github.com/northlighthq/boreal/internal/vault"
)

const maxJumpDepth = 5

func (a *App) DefaultProfile() shell.Profile {
	for _, p := range a.profiles {
		if p.ID == a.settings.Shell {
			return p
		}
	}

	return a.profiles[0]
}

func (a *App) profileMenu() []kit.MenuItem {
	items := make([]kit.MenuItem, len(a.profiles))

	for i, p := range a.profiles {
		items[i] = kit.MenuItem{Label: p.Name, Icon: icons.Terminal}

		if i == 0 {
			items[i].Shortcut = "Ctrl+Shift+T"
		}
	}

	return items
}

func (a *App) termSettings() termview.Settings {
	s := a.settings
	style := vt.CursorBlock

	switch s.CursorStyle {
	case config.CursorBar:
		style = vt.CursorBar
	case config.CursorUnderline:
		style = vt.CursorUnderline
	}

	return termview.Settings{
		TextSize:     unit.Sp(s.FontSize),
		LineHeight:   s.LineHeight,
		CursorStyle:  style,
		CursorBlink:  s.CursorBlink,
		CopyOnSelect: s.CopyOnSelect,
		Ligatures:    s.Ligatures,
		Highlight:    s.Highlight,
	}
}

func (a *App) newPane(s *session.Session) *Pane {
	p := &Pane{sess: s, icon: icons.Terminal, color: a.th().TextMuted}
	p.view = termview.New(s, a.kit, a.glyphs, a.termSettings())
	p.view.OnOpenURL = func(link string) {
		if !openURL(link) {
			a.toasts.Error("Link not opened", "Boreal only opens http and https links.")
		}
	}
	p.view.OnAction = func(action string) { a.paneAction(p, action) }
	p.view.OnFocus = func() { a.focusPane(p) }
	p.view.Expand = a.expandTrigger
	p.view.Extras = func() []termview.Extra { return a.paneExtras(p) }
	return p
}

func (a *App) paneAction(p *Pane, action string) {
	a.focusPane(p)

	switch action {
	case termview.ActionSplitRight:
		a.ws.split(a, true)
	case termview.ActionSplitDown:
		a.ws.split(a, false)
	case termview.ActionClosePane:
		a.ws.closePane(p)
	case actionLog:
		a.toggleLog(p)
	case actionBroadcast:
		a.toggleBroadcast(p)
	case actionCast:
		a.toggleCast(p)
	}
}

func (a *App) focusPane(p *Pane) {
	for i, t := range a.ws.tabs {
		if leaf, _ := t.root.find(p, nil); leaf != nil {
			t.active = p
			a.ws.active = i
			return
		}
	}
}

func (a *App) newLocalPane(pr shell.Profile) *Pane {
	s := session.New(session.Options{
		Kind:        session.Local,
		Command:     pr.Command,
		Cols:        120,
		Rows:        32,
		Scrollback:  a.settings.Scrollback,
		Title:       pr.Name,
		OnChange:    a.Invalidate,
		OnClipboard: a.serverCopy(pr.Name),
	})
	p := a.newPane(s)
	p.origin = origin{Profile: pr.ID, Command: pr.Command, Title: pr.Name}
	return p
}

func (a *App) NewLocalTab(pr shell.Profile) {
	a.ws.addTab(a.newLocalPane(pr))
}

func (a *App) newSSHPane(h vault.Host) *Pane {
	go a.vault.TouchHost(h.ID)
	asker := new(int)

	s := session.New(session.Options{
		Kind:        session.SSH,
		OnClipboard: a.serverCopy(h.Title()),
		Target:      a.target(h, 0),
		Vault:       a.vault,
		Hooks:       a.hooksFor(asker),
		Redact: func(s string) string {
			if r := a.redactor(); r != nil {
				return r(s)
			}

			return s
		},
		KeepAlive:  time.Duration(a.keepAlive.Load()) * time.Second,
		Cols:       120,
		Rows:       32,
		Scrollback: a.settings.Scrollback,
		Title:      h.Title(),
		OnChange:   a.Invalidate,
	})

	p := a.newPane(s)
	p.asker = asker
	p.icon = icons.Server
	p.color = a.th().TextMuted
	p.origin = origin{HostID: h.ID}

	if h.ID == "" {
		quick := h
		quick.Password = ""
		p.origin = origin{Host: &quick}
	}

	return p
}

func (a *App) Connect(h vault.Host) {
	a.ws.addTab(a.newSSHPane(h))
}

func (a *App) target(h vault.Host, depth int) sshx.Target {
	t := sshx.Target{Host: h}

	if h.Auth == vault.AuthKey && h.KeyID != "" {
		if k, ok := a.vault.Key(h.KeyID); ok {
			t.Key = &k
		}
	}

	if h.JumpID != "" && h.JumpID != h.ID && depth < maxJumpDepth {
		if j, ok := a.vault.Host(h.JumpID); ok {
			jt := a.target(j, depth+1)
			t.Jump = &jt
		}
	}

	return t
}

func (a *App) hooks() sshx.Hooks {
	return a.hooksFor(nil)
}

func (a *App) hooksFor(owner *int) sshx.Hooks {
	return sshx.Hooks{
		ConfirmHostKey: func(info sshx.HostKeyInfo) bool {
			r := a.ask(&prompt{kind: promptHostKey, hostKey: info, owner: owner})
			return r.ok
		},
		AskPassword: func(msg string) (string, bool) {
			r := a.ask(&prompt{kind: promptSecret, message: msg, owner: owner})
			return r.text, r.ok
		},
		KeyPassphrase: func(k vault.Key) (string, bool) { return a.keyPassphrase(k, owner) },
	}
}

const (
	promptSecret = iota
	promptHostKey
)

type promptReply struct {
	ok       bool
	text     string
	remember bool
}

type prompt struct {
	a       *App
	kind    int
	title   string
	message string
	offer   bool
	keep    widget.Bool
	hostKey sshx.HostKeyInfo
	reply   chan promptReply
	owner   *int

	dlg     kit.Dialog
	editor  widget.Editor
	ok      widget.Clickable
	cancel  widget.Clickable
	focused bool
}

type promptQueue struct {
	mu    sync.Mutex
	items []*prompt
}

func (q *promptQueue) push(p *prompt) {
	q.mu.Lock()
	q.items = append(q.items, p)
	q.mu.Unlock()
}

func (q *promptQueue) peek() *prompt {
	return q.find(nil)
}

func (q *promptQueue) find(owner *int) *prompt {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, it := range q.items {
		if it.owner == owner {
			return it
		}
	}

	return nil
}

func (q *promptQueue) pop(p *prompt) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, it := range q.items {
		if it == p {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return
		}
	}
}

func (a *App) ask(p *prompt) promptReply {
	p.a = a
	p.reply = make(chan promptReply, 1)
	p.editor.SingleLine = true
	p.editor.Submit = true
	p.editor.Mask = '•'

	a.prompts.push(p)
	a.Invalidate()
	return <-p.reply
}

func (p *prompt) answer(a *App, r promptReply) bool {
	a.prompts.pop(p)
	p.reply <- r
	return true
}

func (p *prompt) Layout(gtx C) bool {
	if p.handle(gtx) {
		return true
	}

	return p.a.kit.Dialog(gtx, &p.dlg, p.opts(gtx)) && p.answer(p.a, promptReply{})
}

func (p *prompt) handle(gtx C) bool {
	a := p.a

	if p.ok.Clicked(gtx) {
		return p.answer(a, promptReply{ok: true, text: p.editor.Text(), remember: p.keep.Value})
	}

	if p.cancel.Clicked(gtx) {
		return p.answer(a, promptReply{})
	}

	for {
		ev, ok := p.editor.Update(gtx)

		if !ok {
			break
		}

		if _, ok := ev.(widget.SubmitEvent); ok {
			return p.answer(a, promptReply{ok: true, text: p.editor.Text(), remember: p.keep.Value})
		}
	}

	if p.kind == promptSecret && !p.focused {
		gtx.Execute(key.FocusCmd{Tag: &p.editor})
		p.focused = true
	}

	return false
}

func (p *prompt) opts(gtx C) kit.DialogOpts {
	a := p.a
	k := a.kit
	th := a.th()
	opts := kit.DialogOpts{Width: 460}

	switch p.kind {
	case promptSecret:
		opts.Title = "Authentication"

		if p.title != "" {
			opts.Title = p.title
		}

		opts.Body = func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return k.Text(gtx, p.message) }),
				layout.Rigid(kit.Gap(10)),
				layout.Rigid(func(gtx C) D {
					hint := "Password"

					if p.offer {
						hint = "Passphrase"
					}

					return k.Field(gtx, &p.editor, kit.FieldOpts{Icon: icons.Lock, Hint: hint})
				}),
				layout.Rigid(func(gtx C) D {
					if !p.offer {
						return D{}
					}

					return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D { return k.Checkbox(gtx, &p.keep, "Remember it in the vault") })
				}),
			)
		}
		opts.Footer = kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &p.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &p.ok, kit.Primary, nil, "OK") },
		)

	case promptHostKey:
		info := p.hostKey
		opts.Title = "Verify server"

		if info.Changed {
			opts.Title = "The server key has changed!"
		}

		opts.Body = func(gtx C) D {
			msg := fmt.Sprintf("This is your first connection to %s. Check that the fingerprint matches the server's.", info.Host)

			if info.Changed {
				msg = fmt.Sprintf("The key for %s doesn't match the saved one. The server may have been reinstalled, or this could be a man-in-the-middle attack.", info.Host)
			}

			rows := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{MaxLines: 4}, msg)
				}),
				layout.Rigid(kit.Gap(12)),
				layout.Rigid(func(gtx C) D { return k.Small(gtx, info.KeyType) }),
				layout.Rigid(kit.Gap(4)),
				layout.Rigid(func(gtx C) D {
					return k.CodeBox(gtx, info.Fingerprint, th.Text)
				}),
			}

			if info.Changed {
				rows = append(rows,
					layout.Rigid(kit.Gap(10)),
					layout.Rigid(func(gtx C) D { return k.Small(gtx, "Previous fingerprint") }),
					layout.Rigid(func(gtx C) D {
						return k.CodeBox(gtx, info.OldFingerprint, th.Danger)
					}),
				)
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}
		kind, label := kit.Primary, "Trust and connect"

		if info.Changed {
			kind, label = kit.Danger, "Replace key"
		}

		opts.Footer = kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &p.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &p.ok, kind, icons.Shield, label) },
		)
	}

	return opts
}

func (a *App) redactor() func(string) string {
	if !a.hideAddrs.Load() {
		return nil
	}

	return privacy.RedactIPs
}
