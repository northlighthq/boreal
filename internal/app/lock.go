package app

import (
	"errors"
	"image"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/plattyssh/platty/internal/app/core"
	"github.com/plattyssh/platty/internal/ui/icons"
	"github.com/plattyssh/platty/internal/ui/kit"
	"github.com/plattyssh/platty/internal/vault"
)

type lockScreen struct {
	password widget.Editor
	unlock   widget.Clickable
	focused  bool
	busy     bool
	err      string
	result   chan error
}

func (a *App) LockVault() {
	if !a.vault.HasPassword() {
		a.toasts.Info("No master password", "Set one in Settings → Security to lock Boreal.")
		return
	}

	a.vault.Lock()
	a.passes.clear()
	a.palette.Close()
	a.modal = nil
	a.docked = [2]core.Modal{}
	a.lock = lockScreen{}
	a.Invalidate()
}

func (a *App) idleLock(gtx C) {
	if a.settings.LockAfter <= 0 || !a.vault.HasPassword() {
		return
	}

	last := a.started

	if t := a.kit.LastActive(); t.After(last) {
		last = t
	}

	for _, t := range a.ws.tabs {
		for _, p := range t.root.panes(nil) {
			if in := p.view.LastInput(); in.After(last) {
				last = in
			}
		}
	}

	deadline := last.Add(time.Duration(a.settings.LockAfter) * time.Minute)

	if gtx.Now.After(deadline) {
		a.LockVault()
		return
	}

	gtx.Execute(op.InvalidateCmd{At: deadline})
}

func (a *App) lockLayout(gtx C) {
	l := &a.lock
	k := a.kit
	th := a.th()
	l.password.SingleLine = true
	l.password.Submit = true
	l.password.Mask = '•'

	select {
	case err := <-l.result:
		l.busy = false

		switch {
		case err == nil:
			l.password.SetText("")
			l.err = ""
		case errors.Is(err, vault.ErrWrongPassword):
			l.err = "That isn't the master password."
		default:
			l.err = err.Error()
		}
	default:
	}

	submit := l.unlock.Clicked(gtx)

	for {
		ev, ok := l.password.Update(gtx)

		if !ok {
			break
		}

		if _, ok := ev.(widget.SubmitEvent); ok {
			submit = true
		}
	}

	if submit && !l.busy && l.password.Text() != "" {
		l.busy, l.err = true, ""
		l.result = make(chan error, 1)
		pass, result := l.password.Text(), l.result

		go func() {
			result <- a.vault.Unlock(pass)
			a.Invalidate()
		}()
	}

	if !l.focused {
		gtx.Execute(key.FocusCmd{Tag: &l.password})
		l.focused = true
	}

	size := gtx.Constraints.Max
	kit.Rect(gtx, image.Rectangle{Max: size}, th.Window)
	a.TitleBar(gtx)

	layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(340))

		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return lockBadge(gtx, th) }),
			layout.Rigid(kit.Gap(18)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeLarge, Weight: font.SemiBold}, "Boreal is locked")
			}),
			layout.Rigid(kit.Gap(6)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Align: text.Middle, MaxLines: 2}, "Enter your master password to open the vault.")
			}),
			layout.Rigid(kit.Gap(20)),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return k.Field(gtx, &l.password, kit.FieldOpts{Icon: icons.Lock, Hint: "Master password", Invalid: l.err != ""})
			}),
			layout.Rigid(func(gtx C) D {
				if l.err == "" {
					return D{}
				}

				return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger}, l.err)
				})
			}),
			layout.Rigid(kit.Gap(16)),
			layout.Rigid(func(gtx C) D {
				label := "Unlock"

				if l.busy {
					label = "Unlocking…"
				}

				return k.Button(gtx, &l.unlock, kit.Primary, nil, label)
			}),
		)
	})
}

func (a *App) TitleBar(gtx C) D {
	a.chrome.handleHeader(a, gtx)

	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(headerHeight))
	gtx.Constraints = layout.Exact(size)
	moveArea(gtx, size)
	layout.Inset{Right: 8, Top: headerHeight - tabHeight}.Layout(gtx, func(gtx C) D {
		return layout.E.Layout(gtx, func(gtx C) D { return a.chrome.windowControls(a, gtx) })
	})

	return D{Size: size}
}
