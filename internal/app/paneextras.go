package app

import (
	"fmt"
	"image"
	"io"
	"slices"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/pointer"
	"gioui.org/op"

	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/termview"
)

const (
	actionLog       = "log"
	actionBroadcast = "broadcast"
	actionCast      = "cast"
)

func (a *App) paneExtras(p *Pane) []termview.Extra {
	log := termview.Extra{Action: actionLog, Item: kit.MenuItem{Label: "Log to file…", Icon: icons.File}}

	if p.sess.Logging() {
		log.Item.Label = "Stop logging"
	}

	t := a.ws.tabOf(p)
	cast := termview.Extra{Action: actionBroadcast, Item: kit.MenuItem{Label: "Broadcast input to this tab", Icon: icons.ArrowsOutline}}

	switch {
	case t != nil && t.broadcast:
		cast.Item.Label = "Stop broadcasting"
		pick := termview.Extra{Action: actionCast, Item: kit.MenuItem{Label: "Leave out of broadcast", Icon: icons.ArrowsOutline}}

		if !p.cast {
			pick.Item.Label = "Include in broadcast"
		}

		return []termview.Extra{log, pick, cast}
	case t == nil || len(t.root.panes(nil)) < 2:
		cast.Item.Disabled = true
	}

	return []termview.Extra{log, cast}
}

func (a *App) toggleLog(p *Pane) {
	if p.sess.Logging() {
		p.sess.StopLog()
		a.toasts.Success("Logging stopped", p.sess.Title())
		return
	}

	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '-'
		}

		return r
	}, p.sess.Title())

	go func() {
		w, err := a.explorer.CreateFile(fmt.Sprintf("%s %s.log", name, time.Now().Format("2006-01-02 1504")))

		if err != nil {
			return
		}

		p.sess.StartLog(w)
		a.toasts.Info("Logging "+p.sess.Title(), "Everything this session prints is saved as plain text.")
		a.Invalidate()
	}()
}

func (w *workspace) tabOf(p *Pane) *Tab {
	for _, t := range w.tabs {
		for _, q := range t.root.panes(nil) {
			if q == p {
				return t
			}
		}
	}

	return nil
}

func (a *App) toggleBroadcast(p *Pane) {
	t := a.ws.tabOf(p)

	if t == nil {
		return
	}

	t.broadcast = !t.broadcast

	if !t.broadcast {
		for _, q := range t.root.panes(nil) {
			q.sess.SetMirror(nil)
			q.mirrored = false
			q.cast = false
		}

		return
	}

	skipped := 0

	for _, q := range t.root.panes(nil) {
		q.cast = !a.protected(q)

		if !q.cast {
			skipped++
		}
	}

	msg := "What you type goes to every pane in this tab. Click a pane's label to leave it out."

	switch {
	case skipped == 1:
		msg = "1 protected pane was left out because of its host tags. Click its label to include it anyway."
	case skipped > 1:
		msg = fmt.Sprintf("%d protected panes were left out because of their host tags. Click a label to include one anyway.", skipped)
	}

	a.toasts.Info("Broadcasting", msg)
	syncBroadcast(t)
}

func (a *App) protected(p *Pane) bool {
	h := p.origin.Host

	if h == nil && p.origin.HostID != "" {
		if v, ok := a.vault.Host(p.origin.HostID); ok {
			h = &v
		}
	}

	if h == nil {
		return false
	}

	skip := strings.FieldsFunc(strings.ToLower(a.settings.BroadcastSkip), func(r rune) bool { return r == ',' || r == ' ' })

	for _, tag := range h.Tags {
		if slices.Contains(skip, strings.ToLower(strings.TrimSpace(tag))) {
			return true
		}
	}

	return false
}

func (a *App) toggleActiveBroadcast() {
	t := a.ws.current()

	if t == nil || t.active == nil {
		return
	}

	if !t.broadcast && len(t.root.panes(nil)) < 2 {
		a.toasts.Info("Nothing to broadcast to", "Split this tab first, then type into every pane at once.")
		return
	}

	a.toggleBroadcast(t.active)
}

func (a *App) toggleCast(p *Pane) {
	if t := a.ws.tabOf(p); t == nil || !t.broadcast {
		return
	}

	p.cast = !p.cast
}

func syncBroadcast(t *Tab) {
	if !t.broadcast {
		return
	}

	for _, p := range t.root.panes(nil) {
		if p.mirrored {
			continue
		}

		p.mirrored = true
		p.sess.SetMirror(func(b []byte) {
			if !p.cast {
				return
			}

			for _, q := range t.root.panes(nil) {
				if q != p && q.cast {
					q.sess.SendOnly(b)
				}
			}
		})
	}
}

func (a *App) broadcastMark(gtx C, p *Pane, size image.Point) {
	th := a.th()

	if p.castLabel.Clicked(gtx) {
		a.toggleCast(p)
	}

	label, c := "Not receiving", th.TextMuted

	if a.protected(p) {
		label = "Protected · not receiving"
	}

	if p.cast {
		kit.Border(gtx, image.Rectangle{Max: size}.Inset(gtx.Dp(3)), gtx.Dp(th.RadiusSmall), float32(gtx.Dp(1.5)), kit.Fade(th.Danger, 0.7))
		label, c = "Broadcasting", th.Danger
	}

	gtx.Constraints.Min = image.Point{}
	m := op.Record(gtx.Ops)
	dims := p.castLabel.Layout(gtx, func(gtx C) D {
		return kit.Background(gtx, th.Background, th.RadiusSmall, func(gtx C) D {
			return a.kit.Badge(gtx, label, c)
		})
	})
	call := m.Stop()

	pad := gtx.Dp(10)
	defer op.Offset(image.Pt(size.X-dims.Size.X-pad, pad)).Push(gtx.Ops).Pop()
	kit.Cursor(gtx, dims.Size, pointer.CursorPointer)
	call.Add(gtx.Ops)
}

const (
	maxServerCopy = 1 << 20
	clipPreview   = 60
)

type serverClip struct {
	from string
	text string
}

func (a *App) serverCopy(from string) func(string) {
	return func(text string) {
		if len(text) > maxServerCopy {
			return
		}

		select {
		case a.clips <- serverClip{from: from, text: text}:
			a.Invalidate()
		default:
		}
	}
}

func (a *App) applyServerCopies(gtx C) {
	if c := a.allowedClip; c != nil {
		a.allowedClip = nil
		a.writeServerClip(gtx, *c)
	}

	for {
		if a.settings.Clipboard == config.ClipboardAsk && a.modal != nil {
			return
		}

		select {
		case c := <-a.clips:
			switch a.settings.Clipboard {
			case config.ClipboardBlock:
			case config.ClipboardAllow:
				a.writeServerClip(gtx, c)
			default:
				a.askServerClip(c)
				return
			}
		default:
			return
		}
	}
}

func (a *App) askServerClip(c serverClip) {
	preview := strings.TrimSpace(strings.SplitN(c.text, "\n", 2)[0])

	if r := []rune(preview); len(r) > clipPreview {
		preview = string(r[:clipPreview]) + "…"
	}

	msg := fmt.Sprintf("A program in %s wants to copy %d characters to your clipboard: “%s”", c.from, len([]rune(c.text)), preview)
	a.Open(dialogs.NewConfirm(a.kit, "Allow clipboard access?", msg, "Copy", func() {
		a.allowedClip = &c
		a.Invalidate()
	}))
}

func (a *App) writeServerClip(gtx C, c serverClip) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(c.text))})
	a.toasts.Info("Copied from the server", fmt.Sprintf("A program in %s copied %d characters to your clipboard.", c.from, len([]rune(c.text))))
}
