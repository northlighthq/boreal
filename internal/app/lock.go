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

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/platform/hello"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mark"
	"github.com/northlighthq/boreal/internal/vault"
)

const lockWidth = 300

type lockScreen struct {
	password widget.Editor
	unlock   widget.Clickable
	hello    widget.Clickable
	focused  bool
	busy     bool
	viaHello bool
	err      string
	result   chan error
}

func (a *App) LockVault() {
	if !a.CanLock() {
		a.toasts.Info("No master password", "Set one in Settings → Security, or set up Windows Hello, to lock Boreal.")
		return
	}

	if err := a.vault.Lock(); err != nil {
		a.toasts.Error("Couldn't lock Boreal", err.Error())
		return
	}

	a.passes.clear()
	a.palette.Close()
	a.modal = nil
	a.docked = [2]core.Modal{}
	a.lock = lockScreen{}
	a.Invalidate()
}

func (a *App) idleLock(gtx C) {
	if a.settings.LockAfter <= 0 || !a.CanLock() {
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
		l.busy, l.viaHello = false, false

		switch {
		case err == nil:
			l.password.SetText("")
			l.err = ""
		case errors.Is(err, vault.ErrWrongPassword):
			l.err = "That isn't the master password."
		case errors.Is(err, hello.ErrCanceled):
			l.err = ""
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

	hasPassword, canHello := a.vault.HasPassword(), a.canHello()

	if canHello && l.hello.Clicked(gtx) {
		a.unlockWithHello()
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
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X-gtx.Dp(32), gtx.Dp(lockWidth))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, lockContent(k, l, hasPassword, canHello)...)
	})
}

func lockContent(k *kit.Kit, l *lockScreen, hasPassword, canHello bool) []layout.FlexChild {
	th := k.Th
	hint := "Enter your master password to continue."

	if canHello {
		hint = "Confirm it's you to continue."
	}

	wide := func(w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return w(gtx)
		})
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return mark.Layout(gtx, 32) }),
		layout.Rigid(kit.Gap(20)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeLarge + 1, Weight: font.SemiBold, Align: text.Middle}, "Boreal is locked")
		}),
		layout.Rigid(kit.Gap(6)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Align: text.Middle, MaxLines: 2}, hint)
		}),
		layout.Rigid(kit.Gap(24)),
	}

	if canHello {
		label := "Unlock with Windows Hello"

		if l.viaHello {
			label = "Waiting for Windows Hello…"
		}

		children = append(children, wide(func(gtx C) D {
			return k.Button(gtx, &l.hello, kit.Primary, icons.LucideFingerprint, label)
		}))
	}

	if canHello && hasPassword {
		children = append(children, layout.Rigid(kit.Gap(16)), wide(func(gtx C) D { return lockDivider(gtx, k) }), layout.Rigid(kit.Gap(16)))
	}

	if hasPassword {
		children = append(children, wide(func(gtx C) D {
			o := kit.FieldOpts{Icon: icons.Lock, Hint: "Master password", Invalid: l.err != ""}

			if canHello {
				o.Trailing = func(gtx C) D {
					return k.IconButtonSized(gtx, &l.unlock, icons.ArrowRight, false, 22, 16, th.TextMuted)
				}
			}

			return k.Field(gtx, &l.password, o)
		}))
	}

	if hasPassword && !canHello {
		label := "Unlock"

		if l.busy {
			label = "Unlocking…"
		}

		children = append(children, layout.Rigid(kit.Gap(12)), wide(func(gtx C) D {
			return k.Button(gtx, &l.unlock, kit.Primary, nil, label)
		}))
	}

	return append(children, layout.Rigid(func(gtx C) D {
		if l.err == "" {
			return D{}
		}

		return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger, Align: text.Middle, MaxLines: 2}, l.err)
		})
	}))
}

func lockDivider(gtx C, k *kit.Kit) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return k.Divider(gtx, false) }),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: k.Th.TextSizeSmall, Color: k.Th.TextFaint}, "or")
			})
		}),
		layout.Flexed(1, func(gtx C) D { return k.Divider(gtx, false) }),
	)
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
