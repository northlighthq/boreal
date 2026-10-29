package settings

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const settingsLabelWidth = unit.Dp(170)

var settingsPages = []struct {
	title string
	icon  *icons.Icon
}{
	{"Appearance", icons.Tune},
	{"Terminal", icons.Terminal},
	{"SSH", icons.Lock},
	{"Security", icons.Shield},
	{"Data", icons.Download},
	{"Keyboard shortcuts", icons.Keyboard},
	{"About", icons.Info},
}

var cursorStyles = []string{config.CursorBlock, config.CursorBar, config.CursorUnderline}

var clipboardModes = []string{config.ClipboardAsk, config.ClipboardAllow, config.ClipboardBlock}

var keymap = [][2]string{
	{"Search everywhere", "Ctrl+Shift+P"},
	{"New local terminal", "Ctrl+Shift+T"},
	{"New host", "Ctrl+Shift+N"},
	{"Close panel", "Ctrl+Shift+W"},
	{"Split right / down", "Ctrl+Shift+D / Ctrl+Shift+E"},
	{"Broadcast input to the tab", "Ctrl+Shift+B"},
	{"Move focus between panes", "Alt+Arrows"},
	{"Next / previous tab", "Ctrl+Tab / Ctrl+Shift+Tab"},
	{"Tool windows", "Alt+1 … Alt+6"},
	{"Copy / paste", "Ctrl+Shift+C / Ctrl+Shift+V"},
	{"Copy (with selection)", "Ctrl+C"},
	{"Find in terminal", "Ctrl+Shift+F"},
	{"Select all", "Ctrl+Shift+A"},
	{"Scroll history", "Shift+PgUp / Shift+PgDn"},
	{"Font size", "Ctrl+= / Ctrl+- / Ctrl+0"},
	{"Open link", "Ctrl+click"},
	{"Settings", "Ctrl+Alt+S · Ctrl+,"},
	{"Lock Boreal", "Ctrl+Shift+L"},
}

type Dialog struct {
	h     core.Host
	sec   security
	about about
	s     config.Settings
	page  int
	dlg   kit.Dialog

	pages    []widget.Clickable
	theme    kit.Select
	cur      kit.Segmented
	clip     kit.Segmented
	shell    kit.Select
	termFont kit.Select
	scheme   kit.Select
	density  kit.Segmented
	corners  kit.Segmented
	tabStyle kit.Segmented
	accents  []widget.Clickable
	accent   string
	tabIcons widget.Bool

	fontSize, lineHeight, scrollback, keepAlive         widget.Editor
	blink, copySel, confirm, hideIPs, restore, ligs, hl widget.Bool
	autoUpdate                                          widget.Bool
	skipTags                                            widget.Editor

	ok, apply, cancel widget.Clickable
	list              layout.List
}

func New(a core.Host) *Dialog {
	d := &Dialog{h: a, s: a.Settings(), pages: make([]widget.Clickable, len(settingsPages))}
	dialogs.SingleLine(&d.fontSize, &d.lineHeight, &d.scrollback, &d.keepAlive, &d.skipTags)
	d.fontSize.Filter = "0123456789."
	d.lineHeight.Filter = "0123456789."
	d.scrollback.Filter = "0123456789"
	d.keepAlive.Filter = "0123456789"

	s := a.Settings()

	for i, n := range theme.Names {
		if n == s.Theme {
			d.theme.Value = i
		}
	}

	for i, m := range clipboardModes {
		if m == s.Clipboard {
			d.clip.Value = i
		}
	}

	for i, c := range cursorStyles {
		if c == s.CursorStyle {
			d.cur.Value = i
		}
	}

	for i, p := range a.Profiles() {
		if p.ID == s.Shell {
			d.shell.Value = i
		}
	}

	d.fontSize.SetText(strconv.FormatFloat(float64(s.FontSize), 'g', -1, 32))
	d.lineHeight.SetText(strconv.FormatFloat(float64(s.LineHeight), 'g', -1, 32))
	d.scrollback.SetText(strconv.Itoa(s.Scrollback))
	d.keepAlive.SetText(strconv.Itoa(s.KeepAlive))
	d.skipTags.SetText(s.BroadcastSkip)
	d.blink.Value = s.CursorBlink
	d.copySel.Value = s.CopyOnSelect
	d.confirm.Value = s.ConfirmClose
	d.hideIPs.Value = s.HideAddresses
	d.restore.Value = s.RestoreTabs
	d.ligs.Value = s.Ligatures
	d.hl.Value = s.Highlight
	d.tabIcons.Value = s.TabIcons
	d.accent = s.Accent
	d.accents = make([]widget.Clickable, len(theme.Accents))
	d.density.Value = index(theme.Densities, s.Density)
	d.corners.Value = index(theme.CornerStyles, s.Corners)
	d.tabStyle.Value = index(tabStyles, s.TabStyle)
	d.scheme.Value = max(0, index(theme.Schemes(), s.TermScheme))
	d.autoUpdate.Value = s.AutoUpdate

	for i, name := range fonts.TerminalFonts {
		if name == s.TerminalFont {
			d.termFont.Value = i
		}
	}

	d.sec.init(s.LockAfter)
	d.list.Axis = layout.Vertical
	return d
}

func (d *Dialog) collect(a core.Host) config.Settings {
	s := d.s
	s.Theme = theme.Names[d.theme.Value]
	s.CursorStyle = cursorStyles[d.cur.Value]
	s.Clipboard = clipboardModes[d.clip.Value]
	s.TerminalFont = fonts.TerminalFonts[d.termFont.Value]

	if d.shell.Value < len(a.Profiles()) {
		s.Shell = a.Profiles()[d.shell.Value].ID
	}

	if v, err := strconv.ParseFloat(d.fontSize.Text(), 32); err == nil && v >= 8 && v <= 40 {
		s.FontSize = float32(v)
	}

	if v, err := strconv.ParseFloat(d.lineHeight.Text(), 32); err == nil && v >= 1 && v <= 2 {
		s.LineHeight = float32(v)
	}

	if v, err := strconv.Atoi(d.scrollback.Text()); err == nil && v >= 0 {
		s.Scrollback = v
	}

	if v, err := strconv.Atoi(d.keepAlive.Text()); err == nil && v >= 0 {
		s.KeepAlive = v
	}

	s.CursorBlink = d.blink.Value
	s.CopyOnSelect = d.copySel.Value
	s.ConfirmClose = d.confirm.Value
	s.HideAddresses = d.hideIPs.Value
	s.RestoreTabs = d.restore.Value
	s.Ligatures = d.ligs.Value
	s.Highlight = d.hl.Value
	s.TabIcons = d.tabIcons.Value
	s.Accent = d.accent
	s.Density = theme.Densities[d.density.Value]
	s.Corners = theme.CornerStyles[d.corners.Value]
	s.TabStyle = tabStyles[d.tabStyle.Value]
	s.TermScheme = ""

	if d.scheme.Value > 0 {
		s.TermScheme = theme.Schemes()[d.scheme.Value]
	}
	s.AutoUpdate = d.autoUpdate.Value
	s.BroadcastSkip = strings.TrimSpace(d.skipTags.Text())
	s.LockAfter = lockAfterMinutes[d.sec.lockAfter.Value]
	return s
}

func (d *Dialog) commit(a core.Host) {
	s := d.collect(a)

	if s == d.s {
		return
	}

	a.UpdateSettings(s)
	d.s = s
	a.Toasts().Success("Settings saved", "Your changes are applied.")
}

func (d *Dialog) Layout(gtx C) bool {
	a := d.h
	d.sec.update(a, gtx)
	k := a.Kit()

	if d.ok.Clicked(gtx) {
		d.commit(a)
		return true
	}

	if d.apply.Clicked(gtx) {
		d.commit(a)
	}

	if d.cancel.Clicked(gtx) {
		return true
	}

	for i := range d.pages {
		if d.pages[i].Clicked(gtx) {
			d.page = i
		}
	}

	return k.Dialog(gtx, &d.dlg, kit.DialogOpts{
		Title: "Settings",
		Width: 820,
		Body: func(gtx C) D {
			h := min(gtx.Constraints.Max.Y, gtx.Dp(480))
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return d.sidebar(a, gtx) }),
				layout.Rigid(kit.Gap(16)),
				layout.Rigid(func(gtx C) D {
					size := image.Pt(gtx.Dp(1), gtx.Constraints.Min.Y)
					kit.Rect(gtx, image.Rectangle{Max: size}, a.Kit().Th.Divider)
					return D{Size: size}
				}),
				layout.Rigid(kit.Gap(28)),
				layout.Flexed(1, func(gtx C) D {
					return d.list.Layout(gtx, 1, func(gtx C, _ int) D { return d.content(a, gtx) })
				}),
			)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &d.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &d.apply, kit.Secondary, nil, "Apply") },
			func(gtx C) D { return k.Button(gtx, &d.ok, kit.Primary, nil, "OK") },
		),
	})
}

func (d *Dialog) sidebar(a core.Host, gtx C) D {
	k := a.Kit()
	th := a.Kit().Th
	w := gtx.Dp(190)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w

	children := make([]layout.FlexChild, len(settingsPages))

	for i, pg := range settingsPages {
		children[i] = layout.Rigid(func(gtx C) D {
			return d.pages[i].Layout(gtx, func(gtx C) D {
				h := gtx.Dp(30)
				size := image.Pt(w, h)

				if d.page == i {
					kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), th.Selection)
				} else if d.pages[i].Hovered() {
					kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), th.Hover)
				}

				gtx.Constraints = layout.Exact(size)
				layout.Inset{Left: 10}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							col := th.TextFaint

							if d.page == i {
								col = th.Accent
							}

							return pg.icon.Layout(gtx, 16, col)
						}),
						layout.Rigid(kit.Gap(8)),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Min.Y = h
							return layout.W.Layout(gtx, func(gtx C) D {
								col := th.TextMuted

								if d.page == i {
									col = th.Text
								}

								return k.Label(gtx, kit.TextStyle{Color: col}, pg.title)
							})
						}),
					)
				})
				kit.Cursor(gtx, size, pointer.CursorPointer)
				return D{Size: size}
			})
		})
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Dialog) content(a core.Host, gtx C) D {
	k := a.Kit()
	th := a.Kit().Th

	row := func(label string, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx C) D {
				return k.FormRow(gtx, label, settingsLabelWidth, w)
			})
		})
	}
	narrow := func(ed *widget.Editor, hint string) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(100)
			gtx.Constraints.Max.X = gtx.Dp(100)
			return k.Field(gtx, ed, kit.FieldOpts{Hint: hint})
		}
	}
	left := func(w layout.Widget) layout.Widget {
		return func(gtx C) D {
			return layout.W.Layout(gtx, w)
		}
	}
	header := layout.Rigid(func(gtx C) D {
		return layout.Inset{Bottom: 18}.Layout(gtx, func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeLarge, Weight: font.Medium}, settingsPages[d.page].title)
		})
	})

	var children []layout.FlexChild

	switch d.page {
	case 0:
		children = []layout.FlexChild{
			header,
			row("Theme", func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
				return k.Select(gtx, &d.theme, theme.Labels)
			}),
			row("Accent color", left(func(gtx C) D { return d.swatches(a, gtx) })),
			row("Density", left(func(gtx C) D { return k.Segmented(gtx, &d.density, []string{"Compact", "Normal", "Comfortable"}) })),
			row("Corners", left(func(gtx C) D { return k.Segmented(gtx, &d.corners, []string{"Sharp", "Normal", "Round"}) })),
			row("Tabs", left(func(gtx C) D { return k.Segmented(gtx, &d.tabStyle, []string{"Classic", "Pill", "Underline"}) })),
			row("Tab icons", left(func(gtx C) D { return k.Toggle(gtx, &d.tabIcons) })),
			row("Terminal colors", func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
				return k.Select(gtx, &d.scheme, theme.Schemes())
			}),
			row("Interface font", left(func(gtx C) D { return k.Muted(gtx, "Geist") })),
			row("Terminal font", func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
				return k.Select(gtx, &d.termFont, fonts.TerminalFonts)
			}),
			row("Hide IP addresses", left(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return k.Toggle(gtx, &d.hideIPs) }),
					layout.Rigid(kit.Gap(12)),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, "For screenshots and streaming")
					}),
				)
			})),
		}
	case 1:
		names := make([]string, len(a.Profiles()))

		for i, p := range a.Profiles() {
			names[i] = p.Name
		}

		children = []layout.FlexChild{
			header,
			row("Default shell", func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
				return k.Select(gtx, &d.shell, names)
			}),
			row("Font size", left(narrow(&d.fontSize, "13"))),
			row("Line height", left(narrow(&d.lineHeight, "1.2"))),
			row("Cursor", left(func(gtx C) D { return k.Segmented(gtx, &d.cur, []string{"Block", "Bar", "Underline"}) })),
			row("Blinking cursor", left(func(gtx C) D { return k.Toggle(gtx, &d.blink) })),
			row("Copy on select", left(func(gtx C) D { return k.Toggle(gtx, &d.copySel) })),
			row("Programs can copy", left(func(gtx C) D { return k.Segmented(gtx, &d.clip, []string{"Ask", "Allow", "Block"}) })),
			row("Reopen tabs on start", left(func(gtx C) D { return k.Toggle(gtx, &d.restore) })),
			row("Font ligatures", left(func(gtx C) D { return k.Toggle(gtx, &d.ligs) })),
			row("Highlight output", left(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return k.Toggle(gtx, &d.hl) }),
					layout.Rigid(kit.Gap(12)),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, "IPs, URLs, paths, dates and errors")
					}),
				)
			})),
			row("Scrollback lines", left(narrow(&d.scrollback, "10000"))),
			row("Leave out of broadcast", func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return k.Field(gtx, &d.skipTags, kit.FieldOpts{Hint: "prod, production"}) }),
					layout.Rigid(kit.Gap(6)),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, "Hosts with these tags start excluded. Click a pane's label to include it anyway.")
					}),
				)
			}),
		}
	case 2:
		agent := "Not detected (enable the “OpenSSH Authentication Agent” service)"

		if sshx.AgentAvailable() {
			agent = "Available"
		}

		children = []layout.FlexChild{
			header,
			row("Keep-alive (seconds)", left(narrow(&d.keepAlive, "30"))),
			row("Confirm on close", left(func(gtx C) D { return k.Toggle(gtx, &d.confirm) })),
			row("SSH agent", left(func(gtx C) D { return k.Muted(gtx, agent) })),
		}
	case 3:
		children = append([]layout.FlexChild{header}, d.securityPage(a, gtx, row, left)...)
	case 4:
		children = append([]layout.FlexChild{header}, d.dataPage(a, row)...)
	case 6:
		children = d.aboutPage(a, gtx)
	case 5:
		children = []layout.FlexChild{header}

		for _, km := range keymap {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D { return k.Text(gtx, km[0]) }),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Mono: true, Size: th.TextSizeSmall}, km[1])
						}),
					)
				})
			}))
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func vaultSummary(a core.Host) string {
	d := a.Vault().Snapshot()
	return fmt.Sprintf("%d hosts, %d keys, %d snippets.", len(d.Hosts), len(d.Keys), len(d.Snippets))
}

type (
	C = core.C
	D = core.D
)

func (d *Dialog) SetPage(i int) {
	d.page = i
}
