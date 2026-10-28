package app

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

func (a *App) dragHover(over bool) {
	a.dragOver.Store(over)
	a.Invalidate()
}

func (a *App) dropText() (title, detail string, ok bool) {
	pane := a.ws.activePane()

	switch {
	case pane == nil:
		return "Open a session first", "Drop files on an SSH session to upload them.", false
	case pane.sess.Kind != session.SSH:
		return "Drop to paste the paths", "They are typed into " + pane.sess.Title() + ".", true
	}

	dir := a.tools.sftp.Dir(pane.sess.ID)

	if dir == "" {
		return "Drop to upload", "Files go to the current folder on " + pane.sess.Title() + ".", true
	}

	return "Drop to upload", "To " + dir + " on " + pane.sess.Title(), true
}

func (a *App) dropOverlay(gtx C) {
	k := a.kit
	th := k.Th
	t := k.Animated(gtx, kit.Key(&a.dragOver, 0), kit.B2F(a.dragOver.Load()), kit.Fast)

	if t <= 0 {
		return
	}

	size := gtx.Constraints.Max
	kit.Rect(gtx, image.Rectangle{Max: size}, kit.Fade(th.Scrim, t))

	title, detail, ok := a.dropText()
	accent := th.Accent

	if !ok {
		accent = th.TextMuted
	}

	pad := gtx.Dp(14)
	zone := image.Rect(pad, pad, size.X-pad, size.Y-pad)
	kit.RRect(gtx, zone, gtx.Dp(th.RadiusLarge), kit.Fade(accent, 0.08*t))
	kit.Border(gtx, zone, gtx.Dp(th.RadiusLarge), 2, kit.Fade(accent, 0.8*t))

	rec := op.Record(gtx.Ops)
	cg := gtx
	cg.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(420), size.Y)}
	dims := layout.UniformInset(24).Layout(cg, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icons.CloudUpload.Layout(gtx, 40, kit.Fade(accent, t)) }),
			layout.Rigid(kit.Gap(12)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeLarge, Weight: font.SemiBold, Color: kit.Fade(th.Text, t)}, title)
			}),
			layout.Rigid(kit.Gap(4)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: kit.Fade(th.TextMuted, t), MaxLines: 2}, detail)
			}),
		)
	})
	body := rec.Stop()

	at := image.Pt((size.X-dims.Size.X)/2, (size.Y-dims.Size.Y)/2)
	off := op.Offset(at).Push(gtx.Ops)
	k.SurfaceRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(th.RadiusLarge), kit.Fade(th.Popup, t), true)
	body.Add(gtx.Ops)
	off.Pop()
}
