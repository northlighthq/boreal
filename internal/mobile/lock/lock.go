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
	"github.com/northlighthq/boreal/internal/common/mark"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Screen struct {
	h      ui.Host
	pass   widget.Editor
	unlock widget.Clickable
	reveal widget.Clickable
	shown  bool
	err    string
	focus  bool
}

func New(h ui.Host) *Screen {
	s := &Screen{h: h, focus: true}
	ui.SetupField(&s.pass, key.HintPassword, true)
	return s
}

func (s *Screen) Refocus() {
	s.focus = true
}

func (s *Screen) Layout(gtx C) D {
	k := s.h.Kit()
	th := k.Th

	for {
		ev, ok := s.pass.Update(gtx)

		if !ok {
			break
		}

		if _, ok := ev.(widget.SubmitEvent); ok {
			s.try()
		}
	}

	if s.unlock.Clicked(gtx) {
		s.try()
	}

	if s.reveal.Clicked(gtx) {
		s.shown = !s.shown
		s.pass.Mask = '•'

		if s.shown {
			s.pass.Mask = 0
		}
	}

	if s.focus {
		s.focus = false
		gtx.Execute(key.FocusCmd{Tag: &s.pass})
		gtx.Execute(key.SoftKeyboardCmd{Show: true})
	}

	gtx.Constraints.Min = gtx.Constraints.Max

	return layout.Inset{Left: 28, Right: 28}.Layout(gtx, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(380))

			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return mark.Paint(gtx, 56, mark.Aurora)
				}),
				layout.Rigid(ui.Gap(24)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: ui.TextBar + 3, Weight: font.SemiBold, Align: text.Middle}, "Boreal is locked")
				}),
				layout.Rigid(ui.Gap(8)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: ui.TextSub, Color: th.TextMuted, Align: text.Middle, MaxLines: 3}, "Enter your master password to open your hosts and keys.")
				}),
				layout.Rigid(ui.Gap(28)),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					ic := icons.Visibility

					if s.shown {
						ic = icons.VisibilityOff
					}

					return ui.Field(gtx, k, &s.pass, ui.FieldOpts{
						Hint:     "Master password",
						Invalid:  s.err != "",
						Help:     s.err,
						Trailing: func(gtx C) D { return ui.IconButton(gtx, k, &s.reveal, ic, th.TextMuted) },
					})
				}),
				layout.Rigid(ui.Gap(14)),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return ui.Button(gtx, k, &s.unlock, ui.Primary, icons.Lock, "Unlock", true)
				}),
			)
		})
	})
}

func (s *Screen) try() {
	if err := s.h.Vault().Unlock(s.pass.Text()); err != nil {
		s.err = "That password isn't right."
		s.pass.SetText("")
		return
	}

	s.err = ""
	s.pass.SetText("")
	s.h.Invalidate()
}
