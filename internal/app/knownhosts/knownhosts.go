package knownhosts

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

type Panel struct {
	h core.Host

	list   kit.ScrollList
	menu   kit.Menu
	rows   map[string]*kit.Row
	known  []vault.KnownHost
	pick   vault.KnownHost
	forget map[string]*widget.Clickable
}

func (p *Panel) Title() string     { return "Known Hosts" }
func (p *Panel) Icon() *icons.Icon { return icons.ShieldOutline }
func (p *Panel) Side() core.Side   { return core.Right }
func (p *Panel) Shortcut() string  { return "Alt+5" }

func (p *Panel) Refresh() {
	a := p.h
	p.known = a.Vault().Snapshot().KnownHosts
}

func (p *Panel) Actions(gtx C) D {
	return D{}
}

func (p *Panel) Body(gtx C) D {
	a := p.h
	k := a.Kit()
	th := a.Kit().Th

	if p.rows == nil {
		p.rows = map[string]*kit.Row{}
	}

	if i, ok := p.menu.Chosen(); ok && i == 0 {
		host := p.pick.Host
		a.Open(dialogs.NewConfirm(a.Kit(), "Forget this host?", "You'll be asked to confirm again the next time you connect to "+host+".", "Forget", func() {
			a.Vault().ForgetHost(host)
		}))
	}

	if len(p.known) == 0 {
		return a.Kit().EmptyState(gtx, icons.Fingerprint, "No verified servers",
			"Servers show up here once you accept their fingerprint.")
	}

	dims := k.List(gtx, &p.list, len(p.known), func(gtx C, i int) D {
		kh := p.known[i]
		r := p.rows[kh.Host]

		if r == nil {
			r = &kit.Row{}
			p.rows[kh.Host] = r
		}

		if ev := r.Update(gtx); ev.Context {
			p.pick = kh
			p.menu.Open([]kit.MenuItem{{Label: "Forget", Icon: icons.Delete, Danger: true}}, ev.At.Add(image.Pt(0, gtx.Dp(4))))
		}

		if p.forget == nil {
			p.forget = map[string]*widget.Clickable{}
		}

		fg := p.forget[kh.Host]

		if fg == nil {
			fg = &widget.Clickable{}
			p.forget[kh.Host] = fg
		}

		if fg.Clicked(gtx) {
			host := kh.Host
			a.Open(dialogs.NewConfirm(a.Kit(), "Forget this host?", "You'll be asked to confirm again the next time you connect to "+host+".", "Forget", func() {
				a.Vault().ForgetHost(host)
			}))
		}

		return k.Row(gtx, r, kit.RowOpts{Height: 62}, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icons.Shield.Layout(gtx, 18, th.TextMuted) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Weight: font.Medium}, kh.Host)
						}),
						layout.Rigid(kit.Gap(4)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: th.TextFaint, Mono: true}, kh.Fingerprint)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if !(r.Hovered() || fg.Hovered()) {
						return D{}
					}

					return k.IconButtonSized(gtx, fg, icons.Delete, false, 26, 15, th.TextMuted)
				}),
			)
		})
	})

	k.Menu(gtx, &p.menu)
	return dims
}

func New(h core.Host) *Panel {
	return &Panel{h: h}
}

type (
	C = core.C
	D = core.D
)
