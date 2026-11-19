package terminal

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/session"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/mobile/snippets"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	topHeight   = unit.Dp(56)
	minFontSize = 9
	maxFontSize = 24
)

type Screen struct {
	h    Host
	s    *Session
	bar  keyBar
	back widget.Clickable
	kbd  widget.Clickable
	more widget.Clickable

	retry  widget.Clickable
	close  widget.Clickable
	paste  bool
	status widget.List
}

func NewScreen(h Host, s *Session) *Screen {
	return &Screen{h: h, s: s}
}

func (t *Screen) Session() *Session { return t.s }

func (t *Screen) Title() string { return t.s.Host.Title() }

func (t *Screen) FullScreen() bool { return true }

func (t *Screen) Layout(gtx C) D {
	k := t.h.Kit()
	th := k.Th
	st := t.s.State()

	if t.back.Clicked(gtx) {
		t.h.Pop()
	}

	if t.kbd.Clicked(gtx) {
		t.s.View.ShowKeyboard()
	}

	if t.more.Clicked(gtx) {
		t.h.ShowSheet(t.menu())
	}

	if t.retry.Clicked(gtx) {
		t.s.Sess.Reconnect()
	}

	if t.close.Clicked(gtx) {
		t.h.Close(t.s)
	}

	if t.paste {
		t.paste = false
		gtx.Execute(clipboard.ReadCmd{Tag: t.s.View})
	}

	size := gtx.Constraints.Max
	kit.Rect(gtx, image.Rectangle{Max: size}, th.Background)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return t.top(gtx, st) }),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max

			switch st {
			case session.Connecting:
				if len(t.s.Sess.Steps()) > 0 {
					return t.connecting(gtx)
				}
			case session.Failed:
				return t.failed(gtx)
			}

			return t.s.View.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if st != session.Connected {
				return D{}
			}

			return t.bar.Layout(gtx, k, t.s)
		}),
	)
}

func (t *Screen) top(gtx C, st session.State) D {
	k := t.h.Kit()
	th := k.Th
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(topHeight))
	kit.Rect(gtx, image.Rectangle{Max: size}, th.Panel)
	kit.Rect(gtx, image.Rect(0, size.Y-max(gtx.Dp(1)/2, 1), size.X, size.Y), th.Divider)
	gtx.Constraints = layout.Exact(size)

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 4}.Layout(gtx, func(gtx C) D {
				return ui.IconButton(gtx, k, &t.back, icons.LucideArrowLeft, th.Text)
			})
		}),
		layout.Rigid(ui.Gap(4)),
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: ui.TextBody + 1, Weight: font.SemiBold, MaxLines: 1}, t.s.Host.Title())
				}),
				layout.Rigid(func(gtx C) D {
					label, c := StatusText(k, t.s)

					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return Dot(gtx, c) }),
						layout.Rigid(ui.Gap(6)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: th.TextMuted, MaxLines: 1}, label)
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			if st != session.Connected {
				return D{}
			}

			return ui.IconButton(gtx, k, &t.kbd, icons.Keyboard, th.Text)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: 4}.Layout(gtx, func(gtx C) D {
				return ui.IconButton(gtx, k, &t.more, icons.LucideMore, th.Text)
			})
		}),
	)
}

func (t *Screen) connecting(gtx C) D {
	k := t.h.Kit()
	th := k.Th
	steps := t.s.Sess.Steps()
	gtx.Execute(op.InvalidateCmd{})

	return layout.Inset{Left: 28, Right: 28}.Layout(gtx, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: ui.TextBar, Weight: font.SemiBold, MaxLines: 1}, "Connecting to "+t.s.Host.Title())
				}),
				layout.Rigid(ui.Gap(18)),
			}

			for i, s := range steps {
				last := i == len(steps)-1
				rows = append(rows, layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								switch {
								case s.Error:
									return icons.Close.Layout(gtx, 18, th.Danger)
								case last:
									return Dot(gtx, th.Warning)
								}

								return icons.Check.Layout(gtx, 18, th.Accent)
							}),
							layout.Rigid(ui.Gap(12)),
							layout.Flexed(1, func(gtx C) D {
								c := th.TextMuted

								if last {
									c = th.Text
								}

								return k.Label(gtx, kit.TextStyle{Size: ui.TextSub, Color: c, MaxLines: 2}, s.Text)
							}),
						)
					})
				}))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	})
}

func (t *Screen) failed(gtx C) D {
	k := t.h.Kit()
	title, msg := "Disconnected", "The connection was closed."

	if err := t.s.Err(); err != nil {
		title, msg = sshx.Explain(err, t.s.Host.Port)
	}

	return ui.Empty(gtx, k, icons.LucideUnplug, title, msg,
		func(gtx C) D { return ui.Button(gtx, k, &t.retry, ui.Primary, icons.Refresh, "Try again", true) },
		func(gtx C) D { return ui.Button(gtx, k, &t.close, ui.Secondary, nil, "Close session", true) },
	)
}

func (t *Screen) menu() *ui.Sheet {
	s := t.h.Settings()
	size := int(s.FontSize)

	return &ui.Sheet{
		Title:    t.s.Host.Title(),
		Subtitle: "Text size " + strconv.Itoa(size),
		Items: []ui.SheetItem{
			{Icon: icons.Paste, Label: "Paste", Do: func() { t.paste = true; t.h.Invalidate() }},
			{Icon: icons.Snippet, Label: "Run a snippet", Do: t.showSnippets},
			{Icon: icons.Add, Label: "Larger text", Do: func() { t.zoom(1) }},
			{Icon: icons.Tune, Label: "Smaller text", Do: func() { t.zoom(-1) }},
			{Icon: icons.Refresh, Label: "Reconnect", Do: func() { t.s.Sess.Reconnect() }},
			{Icon: icons.LucideUnplug, Label: "Disconnect", Danger: true, Do: func() { t.h.Close(t.s) }},
		},
	}
}

func (t *Screen) showSnippets() {
	all := t.h.Vault().Snapshot().Snippets

	if len(all) == 0 {
		t.h.Notify("No snippets yet", "Add them in Settings › Snippets.")
		return
	}

	items := make([]ui.SheetItem, len(all))

	for i, s := range all {
		items[i] = ui.SheetItem{Icon: icons.Snippet, Label: snippets.Label(s), Do: func() {
			t.s.View.SendText(snippets.Payload(s))
		}}
	}

	t.h.ShowSheet(&ui.Sheet{Title: "Run a snippet", Subtitle: "Sent to " + t.s.Host.Title(), Items: items})
}

func (t *Screen) zoom(d float32) {
	s := t.h.Settings()
	s.FontSize = min(max(s.FontSize+d, minFontSize), maxFontSize)
	t.h.UpdateSettings(s)
}

func StatusText(k *kit.Kit, s *Session) (string, color.NRGBA) {
	th := k.Th

	switch s.State() {
	case session.Connecting:
		return "Connecting…", th.Warning
	case session.Connected:
		return "Connected · " + s.Host.Address, th.Success
	case session.Failed:
		return "Connection failed", th.Danger
	}

	return "Disconnected", th.TextFaint
}

func Dot(gtx C, c color.NRGBA) D {
	side := gtx.Dp(8)
	r := image.Rectangle{Max: image.Pt(side, side)}
	kit.RRect(gtx, r, side/2, c)
	return D{Size: r.Max}
}
