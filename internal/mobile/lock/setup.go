package lock

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type Setup struct {
	h     ui.Host
	pass  widget.Editor
	again widget.Editor
	save  widget.Clickable
	list  widget.List
	err   string
	bad   *widget.Editor
}

func (s *Setup) help(ed *widget.Editor) string {
	if s.bad != ed {
		return ""
	}

	return s.err
}

func NewSetup(h ui.Host) *Setup {
	s := &Setup{h: h}
	ui.SetupField(&s.pass, key.HintPassword, true)
	ui.SetupField(&s.again, key.HintPassword, true)
	return s
}

func (s *Setup) Layout(gtx C) D {
	k := s.h.Kit()
	th := k.Th

	if s.save.Clicked(gtx) {
		s.apply()
	}

	gtx.Constraints.Min = gtx.Constraints.Max

	return ui.Page(gtx, &s.list,
		func(gtx C) D {
			return layout.Inset{Top: 40}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						side := gtx.Dp(64)
						r := image.Rectangle{Max: image.Pt(side, side)}
						kit.RRect(gtx, r, gtx.Dp(20), kit.Fade(th.Accent, 0.12))
						gtx.Constraints = layout.Exact(r.Max)
						return layout.Center.Layout(gtx, func(gtx C) D { return icons.Lock.Layout(gtx, 30, th.Accent) })
					}),
					layout.Rigid(ui.Gap(20)),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: ui.TextBar + 3, Weight: font.SemiBold, Align: text.Middle}, "Protect your vault")
					}),
					layout.Rigid(ui.Gap(8)),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: ui.TextSub, Color: th.TextMuted, Align: text.Middle, MaxLines: 5},
							"This device can't keep Boreal's key in its secure storage. Choose a master password so your hosts and keys stay encrypted.")
					}),
				)
			})
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &s.pass, ui.FieldOpts{Label: "Master password", Hint: "At least 10 characters", Invalid: s.bad == &s.pass, Help: s.help(&s.pass)})
		},
		func(gtx C) D {
			return ui.Field(gtx, k, &s.again, ui.FieldOpts{Label: "Repeat it", Invalid: s.bad == &s.again, Help: s.help(&s.again)})
		},
		func(gtx C) D { return ui.Button(gtx, k, &s.save, ui.Primary, icons.Lock, "Set password", true) },
	)
}

func (s *Setup) apply() {
	if err := vault.CheckPassword(s.pass.Text(), s.again.Text()); err != nil {
		s.bad, s.err = ui.PasswordField(err, &s.pass, &s.again), ui.Sentence(err)
		return
	}

	if err := s.h.Vault().SetPassword("", s.pass.Text()); err != nil {
		s.bad, s.err = &s.pass, ui.Sentence(err)
		return
	}

	s.bad, s.err = nil, ""
	s.pass.SetText("")
	s.again.SetText("")
	s.h.Invalidate()
}
