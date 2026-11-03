package app

import (
	"fmt"
	"image"
	"strings"

	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const pasteSample = 3

func (a *App) paneHost(p *Pane) (vault.Host, bool) {
	if p.origin.Host != nil {
		return *p.origin.Host, true
	}

	if p.origin.HostID == "" {
		return vault.Host{}, false
	}

	return a.vault.Host(p.origin.HostID)
}

func (a *App) tabProtected(t *Tab) bool {
	for _, p := range t.root.panes(nil) {
		if a.protected(p) {
			return true
		}
	}

	return false
}

func (a *App) guardPaste(p *Pane, text string) bool {
	lines := strings.Count(strings.TrimRight(text, "\r\n"), "\n") + 1

	if lines < 2 || !a.protected(p) {
		return false
	}

	h, _ := a.paneHost(p)
	preview := strings.Split(strings.TrimRight(text, "\r\n"), "\n")

	if len(preview) > pasteSample {
		preview = append(preview[:pasteSample], "…")
	}

	msg := fmt.Sprintf("%s is a protected host. You're about to paste %d lines:\n%s", h.Title(), lines, strings.Join(preview, "\n"))
	a.Open(dialogs.NewConfirm(a.kit, "Paste into "+h.Title()+"?", msg, "Paste", func() {
		p.view.Paste(text)
		p.view.Focus()
	}))
	a.Invalidate()
	return true
}

func (a *App) protectedBorder(gtx C, p *Pane, size image.Point) {
	if !a.protected(p) {
		return
	}

	th := a.th()
	kit.Border(gtx, image.Rectangle{Max: size}.Inset(gtx.Dp(1)), gtx.Dp(th.RadiusSmall), float32(gtx.Dp(2)), kit.Fade(th.Danger, 0.8))
}

func (a *App) protectedLead(p *Pane) func(gtx C) D {
	if !a.protected(p) {
		return nil
	}

	h, _ := a.paneHost(p)
	th := a.th()

	return func(gtx C) D {
		return a.kit.Badge(gtx, "PROTECTED · "+h.Title(), th.Danger)
	}
}
