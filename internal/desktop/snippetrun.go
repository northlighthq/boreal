package desktop

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/northlighthq/boreal/internal/core/placeholder"
	"github.com/northlighthq/boreal/internal/core/session"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/desktop/dialogs"
)

const (
	maxSnippetHosts = 12
	connectWait     = 2 * time.Minute
	connectPoll     = 100 * time.Millisecond
	promptSettle    = 400 * time.Millisecond
)

func (a *App) RunSnippet(s vault.Snippet, execute bool) {
	p := a.ws.activePane()

	if p == nil {
		a.toasts.Info("No active terminal", "Open a session to run the snippet.")
		return
	}

	a.withAnswers(s, func(answers map[string]string) {
		cmd := terminalText(placeholder.Fill(s.Command, a.paneValues(p), answers))

		if execute {
			cmd = strings.TrimRight(cmd, "\r") + "\r"
		}

		p.view.SendText(cmd)
		p.view.Focus()
	})
}

func (a *App) RunSnippetOn(s vault.Snippet, hostIDs []string) {
	var hosts []vault.Host

	for _, id := range hostIDs {
		if h, ok := a.vault.Host(id); ok {
			hosts = append(hosts, h)
		}
	}

	if len(hosts) == 0 {
		return
	}

	if len(hosts) > maxSnippetHosts {
		a.toasts.Info("Too many hosts", "Run a snippet on up to "+strconv.Itoa(maxSnippetHosts)+" hosts at once.")
		return
	}

	a.withAnswers(s, func(answers map[string]string) {
		panes := make([]*Pane, len(hosts))

		for i, h := range hosts {
			panes[i] = a.newSSHPane(h)
			cmd := terminalText(placeholder.Fill(s.Command, hostValues(h), answers))
			a.sendWhenConnected(panes[i], strings.TrimRight(cmd, "\r")+"\r")
		}

		a.ws.addGrid(panes)
	})
}

func (a *App) withAnswers(s vault.Snippet, run func(map[string]string)) {
	asks := placeholder.Asks(s.Command)

	if len(asks) == 0 {
		run(nil)
		return
	}

	fields := make([]*dialogs.Field, len(asks))

	for i, q := range asks {
		fields[i] = dialogs.NewField(q, "", "")
	}

	f := dialogs.NewForm(a.kit, s.Label, "Run", fields...)
	f.Subtitle = "This snippet needs a few values before it runs."
	f.OnOK = func(values []string) bool {
		answers := make(map[string]string, len(asks))

		for i, q := range asks {
			answers[q] = values[i]
		}

		run(answers)
		return true
	}

	a.Open(f)
}

func (a *App) paneValues(p *Pane) placeholder.Values {
	if p.origin.HostID != "" {
		if h, ok := a.vault.Host(p.origin.HostID); ok {
			return hostValues(h)
		}
	}

	if p.origin.Host != nil {
		return hostValues(*p.origin.Host)
	}

	name, _ := os.Hostname()
	v := placeholder.Values{Host: name, Label: name}

	if u, err := user.Current(); err == nil {
		_, short, found := strings.Cut(u.Username, `\`)
		v.User = u.Username

		if found {
			v.User = short
		}
	}

	return v
}

func hostValues(h vault.Host) placeholder.Values {
	port := h.Port

	if port == 0 {
		port = 22
	}

	return placeholder.Values{Host: h.Address, User: sshx.User(h), Port: strconv.Itoa(port), Label: h.Title()}
}

func (a *App) sendWhenConnected(p *Pane, cmd string) {
	go func() {
		deadline := time.Now().Add(connectWait)

		for time.Now().Before(deadline) {
			st, _ := p.sess.State()

			switch st {
			case session.Connected:
				time.Sleep(promptSettle)
				p.sess.Send([]byte(cmd))
				return
			case session.Closed, session.Failed:
				return
			}

			time.Sleep(connectPoll)
		}
	}()
}

func terminalText(cmd string) string {
	cmd = strings.ReplaceAll(cmd, "\r\n", "\n")
	return strings.ReplaceAll(cmd, "\n", "\r")
}
