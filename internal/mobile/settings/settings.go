package settings

import (
	"image"
	"image/color"
	"slices"
	"strconv"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/theme"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/core/version"
	"github.com/northlighthq/boreal/internal/mobile/snippets"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	minFont = 9
	maxFont = 24
)

type Root struct {
	h          ui.Host
	list       widget.List
	appearance widget.Clickable
	terminal   widget.Clickable
	security   widget.Clickable
	snippets   widget.Clickable
	hideIPs    widget.Bool
	about      widget.Clickable
}

func NewRoot(h ui.Host) *Root {
	return &Root{h: h}
}

func (r *Root) Title() string { return "Settings" }

func (r *Root) Layout(gtx C) D {
	k := r.h.Kit()
	s := r.h.Settings()
	r.hideIPs.Value = s.HideAddresses

	if r.hideIPs.Update(gtx) {
		s.HideAddresses = r.hideIPs.Value
		r.h.UpdateSettings(s)
	}

	switch {
	case r.appearance.Clicked(gtx):
		r.h.Push(NewAppearance(r.h))
	case r.terminal.Clicked(gtx):
		r.h.Push(NewTerminal(r.h))
	case r.snippets.Clicked(gtx):
		r.h.Push(snippets.NewList(r.h))
	case r.security.Clicked(gtx):
		r.h.Push(NewSecurity(r.h))
	case r.about.Clicked(gtx):
		r.h.Push(NewAbout(r.h))
	}

	themeLabel := theme.Labels[max(slices.Index(theme.Names, s.Theme), 0)]
	lock := "Off"

	if r.h.Vault().HasPassword() {
		lock = "On"
	}

	return ui.Page(gtx, &r.list,
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D {
					return ui.Row(gtx, k, &r.appearance, ui.RowOpts{Lead: tile(k, icons.Tune), Title: "Appearance", Value: themeLabel, Chevron: true})
				},
				func(gtx C) D {
					return ui.Row(gtx, k, &r.terminal, ui.RowOpts{Lead: tile(k, icons.Terminal), Title: "Terminal", Value: strconv.Itoa(int(s.FontSize)) + " pt", Chevron: true})
				},
				func(gtx C) D {
					n := len(r.h.Data().Snippets)
					return ui.Row(gtx, k, &r.snippets, ui.RowOpts{Lead: tile(k, icons.Snippet), Title: "Snippets", Value: strconv.Itoa(n), Chevron: true})
				},
			)
		},
		func(gtx C) D {
			return ui.Section(gtx, k, "Privacy",
				func(gtx C) D {
					return ui.Row(gtx, k, &r.security, ui.RowOpts{Lead: tile(k, icons.Lock), Title: "Master password", Value: lock, Chevron: true})
				},
				func(gtx C) D {
					return ui.Row(gtx, k, nil, ui.RowOpts{
						Lead:     tile(k, icons.VisibilityOff),
						Title:    "Hide IP addresses",
						Subtitle: "For screenshots and screen sharing",
						Trailing: func(gtx C) D { return k.Toggle(gtx, &r.hideIPs) },
					})
				},
			)
		},
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D {
					return ui.Row(gtx, k, &r.about, ui.RowOpts{Lead: tile(k, icons.Info), Title: "About Boreal", Value: version.Version, Chevron: true})
				},
			)
		},
	)
}

func tile(k *kit.Kit, ic *icons.Icon) ui.W {
	return func(gtx C) D {
		th := k.Th
		return ui.Tile(gtx, ic, th.TextMuted, th.Window)
	}
}

type Appearance struct {
	h      ui.Host
	list   widget.List
	themes []widget.Clickable
	font   ui.Segmented
}

func NewAppearance(h ui.Host) *Appearance {
	a := &Appearance{h: h, themes: make([]widget.Clickable, len(theme.Names))}
	a.font.Value = max(slices.Index(fonts.TerminalFonts, h.Settings().TerminalFont), 0)
	return a
}

func (a *Appearance) Title() string { return "Appearance" }

func (a *Appearance) Layout(gtx C) D {
	k := a.h.Kit()
	s := a.h.Settings()

	for i := range a.themes {
		if a.themes[i].Clicked(gtx) && s.Theme != theme.Names[i] {
			s.Theme = theme.Names[i]
			a.h.UpdateSettings(s)
		}
	}

	if a.font.Changed(gtx) {
		s.TerminalFont = fonts.TerminalFonts[a.font.Value]
		a.h.UpdateSettings(s)
	}

	return ui.Page(gtx, &a.list,
		func(gtx C) D { return a.grid(gtx, k, s.Theme) },
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return caption(gtx, k, "Terminal font") }),
				layout.Rigid(func(gtx C) D { return a.font.Layout(gtx, k, fonts.TerminalFonts) }),
			)
		},
	)
}

func (a *Appearance) grid(gtx C, k *kit.Kit, current string) D {
	const cols = 3
	gap := gtx.Dp(10)
	w := (gtx.Constraints.Max.X - gap*(cols-1)) / cols
	var rows []layout.FlexChild

	rows = append(rows, layout.Rigid(func(gtx C) D { return caption(gtx, k, "Theme") }))

	for start := 0; start < len(theme.Names); start += cols {
		rows = append(rows, layout.Rigid(func(gtx C) D {
			var cells []layout.FlexChild

			for i := start; i < min(start+cols, len(theme.Names)); i++ {
				if i > start {
					cells = append(cells, layout.Rigid(func(gtx C) D { return D{Size: image.Pt(gap, 0)} }))
				}

				cells = append(cells, layout.Rigid(func(gtx C) D {
					gtx.Constraints = layout.Exact(image.Pt(w, gtx.Dp(118)))
					return a.swatch(gtx, k, i, theme.Names[i] == current)
				}))
			}

			return layout.Inset{Bottom: unit.Dp(gap)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx, cells...)
			})
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (a *Appearance) swatch(gtx C, k *kit.Kit, i int, on bool) D {
	th := k.Th
	t := theme.New(theme.Names[i])
	clk := &a.themes[i]

	return clk.Layout(gtx, func(gtx C) D {
		size := gtx.Constraints.Max
		preview := image.Rect(0, 0, size.X, size.Y-gtx.Dp(30))
		radius := gtx.Dp(14)
		kit.RRect(gtx, preview, radius, t.Window)

		line := func(y, w int, c color.NRGBA) {
			x := gtx.Dp(14)
			r := image.Rect(x, y, x+w, y+gtx.Dp(6))
			kit.RRect(gtx, r, r.Dy()/2, c)
		}

		inner := preview.Dx() - gtx.Dp(28)
		line(gtx.Dp(18), inner*2/3, t.Accent)
		line(gtx.Dp(32), inner, t.TextMuted)
		line(gtx.Dp(46), inner*3/4, t.TextFaint)
		line(gtx.Dp(60), inner/2, t.TextMuted)

		border := kit.Fade(th.Border, 0.9)
		width := float32(gtx.Dp(1))

		if on {
			border, width = th.Accent, float32(gtx.Dp(2))
		}

		kit.Border(gtx, preview, radius, width, border)

		gtx.Constraints = layout.Exact(image.Pt(size.X, gtx.Dp(26)))
		layoutOffset(gtx, preview.Max.Y+gtx.Dp(4), func(gtx C) D {
			w := font.Normal
			c := th.TextMuted

			if on {
				w, c = font.SemiBold, th.Text
			}

			return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: c, Weight: w, MaxLines: 1, Align: text.Middle}, theme.Labels[i])
		})
		return D{Size: size}
	})
}

type Terminal struct {
	h      ui.Host
	list   widget.List
	bigger widget.Clickable
	small  widget.Clickable
	cursor ui.Segmented
	blink  widget.Bool
}

var cursorNames = []string{config.CursorBlock, config.CursorBar, config.CursorUnderline}

func NewTerminal(h ui.Host) *Terminal {
	t := &Terminal{h: h}
	t.cursor.Value = max(slices.Index(cursorNames, h.Settings().CursorStyle), 0)
	return t
}

func (t *Terminal) Title() string { return "Terminal" }

func (t *Terminal) Layout(gtx C) D {
	k := t.h.Kit()
	th := k.Th
	s := t.h.Settings()
	t.blink.Value = s.CursorBlink

	changed := false

	if t.bigger.Clicked(gtx) && s.FontSize < maxFont {
		s.FontSize++
		changed = true
	}

	if t.small.Clicked(gtx) && s.FontSize > minFont {
		s.FontSize--
		changed = true
	}

	if t.cursor.Changed(gtx) {
		s.CursorStyle = cursorNames[t.cursor.Value]
		changed = true
	}

	if t.blink.Update(gtx) {
		s.CursorBlink = t.blink.Value
		changed = true
	}

	if changed {
		t.h.UpdateSettings(s)
	}

	return ui.Page(gtx, &t.list,
		func(gtx C) D { return termPreview(gtx, k, s) },
		func(gtx C) D {
			return ui.Section(gtx, k, "Text",
				func(gtx C) D {
					return ui.Row(gtx, k, nil, ui.RowOpts{
						Title: "Text size",
						Trailing: func(gtx C) D {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return ui.IconButton(gtx, k, &t.small, icons.Minus, th.Text) }),
								layout.Rigid(func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Dp(36)
									return layout.Center.Layout(gtx, func(gtx C) D {
										return k.Label(gtx, kit.TextStyle{Size: ui.TextBody, Weight: font.SemiBold}, strconv.Itoa(int(s.FontSize)))
									})
								}),
								layout.Rigid(func(gtx C) D { return ui.IconButton(gtx, k, &t.bigger, icons.Add, th.Text) }),
							)
						},
					})
				},
				func(gtx C) D {
					return ui.Row(gtx, k, nil, ui.RowOpts{Title: "Blinking cursor", Trailing: func(gtx C) D { return k.Toggle(gtx, &t.blink) }})
				},
			)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return caption(gtx, k, "Cursor") }),
				layout.Rigid(func(gtx C) D { return t.cursor.Layout(gtx, k, []string{"Block", "Bar", "Underline"}) }),
			)
		},
	)
}

type Security struct {
	h       ui.Host
	list    widget.List
	current widget.Editor
	next    widget.Editor
	confirm widget.Editor
	save    widget.Clickable
	remove  widget.Clickable
	err     string
	bad     *widget.Editor
}

func NewSecurity(h ui.Host) *Security {
	s := &Security{h: h}
	ui.SetupField(&s.current, key.HintPassword, true)
	ui.SetupField(&s.next, key.HintPassword, true)
	ui.SetupField(&s.confirm, key.HintPassword, true)
	return s
}

func (s *Security) Title() string { return "Master password" }

func (s *Security) Layout(gtx C) D {
	k := s.h.Kit()
	has := s.h.Vault().HasPassword()

	if s.save.Clicked(gtx) {
		s.apply(has, false)
	}

	if s.remove.Clicked(gtx) {
		s.apply(has, true)
	}

	children := []ui.W{
		func(gtx C) D {
			msg := "Set a master password to encrypt your hosts and keys with it. Boreal asks for it every time it opens."

			if has {
				msg = "Your vault is locked with a master password. Boreal asks for it every time it opens."
			}

			return ui.Caption(gtx, k, msg)
		},
	}

	rows := []ui.W{}

	if has {
		rows = append(rows, func(gtx C) D {
			return ui.FormRow(gtx, k, "Current", &s.current, ui.FormOpts{Hint: "Password", Invalid: s.bad == &s.current})
		})
	}

	rows = append(rows,
		func(gtx C) D {
			return ui.FormRow(gtx, k, "New", &s.next, ui.FormOpts{Hint: "At least 10 characters", Invalid: s.bad == &s.next})
		},
		func(gtx C) D {
			return ui.FormRow(gtx, k, "Repeat", &s.confirm, ui.FormOpts{Hint: "Same again", Invalid: s.bad == &s.confirm})
		},
	)

	children = append(children, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ui.Section(gtx, k, "", rows...) }),
			layout.Rigid(func(gtx C) D {
				if s.err == "" {
					return D{}
				}

				return layout.Inset{Left: 4, Top: 8}.Layout(gtx, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: ui.TextCaption, Color: k.Th.Danger, MaxLines: 3}, s.err)
				})
			}),
		)
	})

	label := "Set password"

	if has {
		label = "Change password"
	}

	actions := []ui.W{func(gtx C) D { return ui.ActionRow(gtx, k, &s.save, icons.Lock, label, ui.Primary) }}

	if has {
		actions = append(actions, func(gtx C) D { return ui.ActionRow(gtx, k, &s.remove, icons.Delete, "Remove password", ui.Destructive) })
	}

	children = append(children, func(gtx C) D { return ui.Section(gtx, k, "", actions...) })

	return ui.Page(gtx, &s.list, children...)
}

func (s *Security) apply(has, remove bool) {
	next := s.next.Text()

	if !remove {
		if err := vault.CheckPassword(next, s.confirm.Text()); err != nil {
			s.bad, s.err = ui.PasswordField(err, &s.next, &s.confirm), ui.Sentence(err)
			return
		}
	} else {
		next = ""
	}

	if err := s.h.Vault().SetPassword(s.current.Text(), next); err != nil {
		s.bad, s.err = &s.current, "The current password isn't right."
		return
	}

	if !remove {
		if err := s.h.Vault().SetOSUnlock(false); err != nil {
			s.h.Notify("Couldn't require the password", ui.Sentence(err))
			return
		}
	}

	s.bad, s.err = nil, ""

	switch {
	case remove:
		s.h.Notify("Password removed", "Boreal opens without asking for it.")
	case has:
		s.h.Notify("Password changed", "Use the new one next time.")
	default:
		s.h.Notify("Password set", "Boreal will ask for it when it opens.")
	}

	s.h.Pop()
}

type About struct {
	h    ui.Host
	list widget.List
}

func NewAbout(h ui.Host) *About { return &About{h: h} }

func (a *About) Title() string { return "About" }

func (a *About) Layout(gtx C) D {
	k := a.h.Kit()

	return ui.Page(gtx, &a.list,
		func(gtx C) D {
			return ui.Section(gtx, k, "",
				func(gtx C) D { return ui.Row(gtx, k, nil, ui.RowOpts{Title: "Version", Value: version.Version}) },
				func(gtx C) D { return ui.Row(gtx, k, nil, ui.RowOpts{Title: "License", Value: "MIT"}) },
				func(gtx C) D { return ui.Row(gtx, k, nil, ui.RowOpts{Title: "Made by", Value: "Northlight"}) },
			)
		},
		func(gtx C) D {
			return ui.Caption(gtx, k, "Boreal is open source. The embedded fonts are under the SIL Open Font License and the icons come from Lucide, Tabler and Phosphor.")
		},
	)
}

func caption(gtx C, k *kit.Kit, s string) D {
	return ui.Heading(gtx, k, s)
}

func layoutOffset(gtx C, y int, w ui.W) {
	defer opOffset(gtx, y).Pop()
	w(gtx)
}

func opOffset(gtx C, y int) op.TransformStack {
	return op.Offset(image.Pt(0, y)).Push(gtx.Ops)
}
