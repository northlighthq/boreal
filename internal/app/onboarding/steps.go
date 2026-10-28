package onboarding

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
	"github.com/northlighthq/boreal/internal/vault"
)

func (d *Intro) welcome(gtx C) D {
	k := d.h.Kit()
	th := k.Th

	point := func(ic *icons.Icon, text string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 18, th.TextMuted) }),
					layout.Rigid(kit.Gap(12)),
					layout.Flexed(1, func(gtx C) D { return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 2}, text) }),
				)
			})
		})
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		point(icons.Server, "Keep your servers in folders and connect with a double click."),
		point(icons.Terminal, "A fast terminal of its own, with tabs and split panes."),
		point(icons.Key, "Keys, port forwarding, SFTP and snippets, all in one window."),
		point(icons.Lock, "Everything stays encrypted on this PC."),
	)
}

func (d *Intro) look(gtx C) D {
	k := d.h.Kit()
	current := d.h.Settings().Theme
	const perRow = 4
	var rows []layout.FlexChild

	for start := 0; start < len(theme.Names); start += perRow {
		cards := make([]layout.FlexChild, 0, 2*perRow)

		for i := start; i < start+perRow; i++ {
			if i > start {
				cards = append(cards, layout.Rigid(kit.Gap(10)))
			}

			if i >= len(theme.Names) {
				cards = append(cards, layout.Flexed(1, func(gtx C) D { return D{} }))
				continue
			}

			name := theme.Names[i]
			cards = append(cards, layout.Flexed(1, func(gtx C) D {
				return d.themeCard(gtx, &d.themes[i], name, theme.Labels[i], name == current)
			}))
		}

		if start > 0 {
			rows = append(rows, layout.Rigid(kit.Gap(12)))
		}

		rows = append(rows, layout.Rigid(func(gtx C) D { return layout.Flex{}.Layout(gtx, cards...) }))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "THEME") }),
		layout.Rigid(kit.Gap(10)),
		layout.Rigid(func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...) }),
		layout.Rigid(kit.Gap(22)),
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "TERMINAL FONT") }),
		layout.Rigid(kit.Gap(10)),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(260))
			return k.Select(gtx, &d.termFont, fonts.TerminalFonts)
		}),
	)
}

func (d *Intro) themeCard(gtx C, clk *widget.Clickable, name, label string, selected bool) D {
	k := d.h.Kit()
	th := k.Th
	t := theme.New(name)

	return clk.Layout(gtx, func(gtx C) D {
		w := gtx.Constraints.Max.X
		h := gtx.Dp(70)
		r := image.Rect(0, 0, w, h)
		radius := gtx.Dp(th.Radius)
		kit.RRect(gtx, r, radius, t.Frame)
		inner := image.Rect(gtx.Dp(22), gtx.Dp(10), w-gtx.Dp(6), h-gtx.Dp(6))
		kit.RRect(gtx, inner, gtx.Dp(th.RadiusSmall), t.Editor)

		for i, frac := range []float32{0.7, 0.45, 0.55} {
			y := inner.Min.Y + gtx.Dp(8) + i*gtx.Dp(10)
			x0 := inner.Min.X + gtx.Dp(8)
			x1 := x0 + int(float32(inner.Dx()-gtx.Dp(16))*frac)
			kit.RRect(gtx, image.Rect(x0, y, x1, y+gtx.Dp(4)), gtx.Dp(2), kit.Fade(previewLine(t, i), 0.5+0.2*float32(1-i%2)))
		}

		for i := range 3 {
			y := gtx.Dp(14) + i*gtx.Dp(10)
			kit.RRect(gtx, image.Rect(gtx.Dp(7), y, gtx.Dp(16), y+gtx.Dp(4)), gtx.Dp(2), kit.Fade(t.TextMuted, 0.6))
		}

		line, width := th.Border, float32(1)

		switch {
		case selected:
			line, width = th.Text, 2
		case clk.Hovered():
			line = th.TextMuted
		}

		kit.Border(gtx, r, radius, width, line)

		labelDims := layout.Inset{Top: unit.Dp(float32(h)/gtx.Metric.PxPerDp + 8)}.Layout(gtx, func(gtx C) D {
			weight := font.Normal

			if selected {
				weight = font.SemiBold
			}

			return k.Label(gtx, kit.TextStyle{Weight: weight, Size: th.TextSizeSmall}, label)
		})

		return D{Size: image.Pt(w, labelDims.Size.Y)}
	})
}

func (d *Intro) terminal(gtx C) D {
	k := d.h.Kit()
	names := make([]string, len(d.h.Profiles()))

	for i, p := range d.h.Profiles() {
		names[i] = p.Name
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "DEFAULT SHELL") }),
		layout.Rigid(kit.Gap(10)),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(300))
			return k.Select(gtx, &d.shell, names)
		}),
		layout.Rigid(kit.Gap(22)),
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "CURSOR") }),
		layout.Rigid(kit.Gap(10)),
		layout.Rigid(func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				return k.Segmented(gtx, &d.cursor, []string{"Block", "Bar", "Underline"})
			})
		}),
	)
}

func (d *Intro) security(gtx C) D {
	k := d.h.Kit()
	th := k.Th

	if d.h.Vault().HasPassword() {
		return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 3}, "Your vault already has a master password. You can change it in Settings → Security.")
	}

	field := func(ed *widget.Editor, hint string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(360))
				return k.Field(gtx, ed, kit.FieldOpts{Icon: icons.Lock, Hint: hint, Invalid: d.passErr != ""})
			})
		})
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return k.SectionTitle(gtx, "MASTER PASSWORD · OPTIONAL") }),
		layout.Rigid(kit.Gap(10)),
		field(&d.password, fmt.Sprintf("At least %d characters, or leave empty", minPassword)),
		field(&d.confirm, "Repeat it"),
	}

	if d.passErr != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger}, d.passErr)
			})
		}))
	}

	children = append(children,
		layout.Rigid(kit.Gap(6)),
		layout.Rigid(func(gtx C) D {
			return k.Checkbox(gtx, &d.osUnlock, "Also open the vault with my "+vault.OSOwner+", without typing it")
		}),
		layout.Rigid(kit.Gap(12)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, "If you forget the master password and turn the "+vault.OSName+" option off, Boreal can't open the vault again.")
		}),
	)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Intro) servers(gtx C) D {
	k := d.h.Kit()
	th := k.Th

	option := func(clk *widget.Clickable, ic *icons.Icon, title, text, button string, done bool) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D {
				m := op.Record(gtx.Ops)
				dims := layout.UniformInset(14).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 20, th.TextMuted) }),
						layout.Rigid(kit.Gap(14)),
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Weight: font.Medium}, title) }),
								layout.Rigid(kit.Gap(2)),
								layout.Rigid(func(gtx C) D {
									return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 2}, text)
								}),
							)
						}),
						layout.Rigid(kit.Gap(12)),
						layout.Rigid(func(gtx C) D {
							if done {
								return icons.Check.Layout(gtx, 18, th.Text)
							}

							return k.Button(gtx, clk, kit.Secondary, nil, button)
						}),
					)
				})
				call := m.Stop()
				r := image.Rectangle{Max: dims.Size}
				kit.Border(gtx, r, gtx.Dp(th.Radius), 1, th.Border)
				call.Add(gtx.Ops)
				return dims
			})
		})
	}

	var children []layout.FlexChild

	if d.sshHosts > 0 {
		children = append(children, option(&d.importSSH, icons.DownloadOutline, "Import ~/.ssh/config",
			fmt.Sprintf("I found %d hosts there. Their keys come along too.", d.sshHosts), "Import", d.imported))
	}

	children = append(children,
		option(&d.addHost, icons.PlusOutline, "Add a host", "Type the address, user and how to log in.", "Add", d.openEditor),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 2}, "You can also skip this: local terminals work right away, and Ctrl+Shift+P connects to any user@host.")
		}),
	)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Intro) done(gtx C) D {
	k := d.h.Kit()
	th := k.Th
	var rows []layout.FlexChild

	for _, s := range shortcuts {
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D { return k.Text(gtx, s[0]) }),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Mono: true, Size: th.TextSizeSmall}, s[1])
					}),
				)
			})
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func previewLine(t *theme.Theme, i int) color.NRGBA {
	switch {
	case !t.Colorful():
		return t.Text
	case i == 0:
		return t.Accent
	case i == 2:
		return t.Success
	}

	return t.Text
}
