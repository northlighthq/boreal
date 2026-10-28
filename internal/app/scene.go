package app

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"

	"github.com/plattyssh/platty/internal/ui/icons"
	"github.com/plattyssh/platty/internal/ui/kit"
	"github.com/plattyssh/platty/internal/ui/mark"
	"github.com/plattyssh/platty/internal/ui/theme"
)

func lockBadge(gtx C, th *theme.Theme) D {
	box := gtx.Dp(56)
	r := image.Rect(0, 0, box, box)
	kit.RRect(gtx, r, gtx.Dp(th.RadiusLarge+2), th.Selection)
	kit.Border(gtx, r, gtx.Dp(th.RadiusLarge+2), 1, th.Border)

	px := gtx.Dp(24)
	off := op.Offset(image.Pt((box-px)/2, (box-px)/2)).Push(gtx.Ops)
	icons.Lock.Layout(gtx, 24, th.Text)
	off.Pop()

	return D{Size: image.Pt(box, box)}
}

func (a *App) stepList(gtx C, p *Pane, live bool) D {
	k := a.kit
	th := a.th()
	steps := p.sess.Steps()

	if len(steps) > 0 {
		steps = steps[1:]
	}

	rowH := gtx.Dp(30)
	var rows []layout.FlexChild

	for i, st := range steps {
		current := live && i == len(steps)-1
		last := i == len(steps)-1
		rows = append(rows, layout.Rigid(func(gtx C) D {
			x := easeOut(gtx.Now.Sub(st.At).Seconds() / 0.35)

			if x < 1 {
				gtx.Execute(op.InvalidateCmd{})
			}

			rec := op.Record(gtx.Ops)
			px := gtx.Dp(16)
			top := (rowH - px) / 2

			if !last {
				cx := px / 2
				kit.Rect(gtx, image.Rect(cx, top+px+gtx.Dp(4), cx+max(gtx.Dp(1), 1), rowH+top-gtx.Dp(4)), th.Divider)
			}

			off := op.Offset(image.Pt(0, top)).Push(gtx.Ops)

			switch {
			case st.Error:
				icons.Close.Layout(gtx, 16, th.Danger)
			case current:
				spinner(gtx, px, th.Accent, gtx.Now)
			default:
				icons.Check.Layout(gtx, 16, th.Accent)
			}

			off.Pop()

			col := th.TextMuted

			switch {
			case st.Error:
				col = th.Danger
			case current:
				col = th.Text
			}

			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X-px-gtx.Dp(12), rowH)}
			lrec := op.Record(gtx.Ops)
			ld := k.Label(g, kit.TextStyle{Color: col, MaxLines: 1}, st.Text)
			label := lrec.Stop()
			off = op.Offset(image.Pt(px+gtx.Dp(12), (rowH-ld.Size.Y)/2)).Push(gtx.Ops)
			label.Add(gtx.Ops)
			off.Pop()
			call := rec.Stop()

			size := image.Pt(gtx.Constraints.Max.X, rowH)
			defer op.Offset(image.Pt(0, int(float32(gtx.Dp(6))*(1-x)))).Push(gtx.Ops).Pop()
			defer paint.PushOpacity(gtx.Ops, max(x, 0.01)).Pop()
			call.Add(gtx.Ops)
			return D{Size: size}
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (a *App) brandStage(gtx C, opacity float32, live bool, title string, titleColor color.NRGBA, steps, extra layout.Widget) {
	k := a.kit
	th := a.th()
	size := gtx.Constraints.Max

	if opacity < 1 {
		defer paint.PushOpacity(gtx.Ops, max(opacity, 0.01)).Pop()
	}

	kit.Rect(gtx, image.Rectangle{Max: size}, th.Editor)
	g := gtx
	g.Constraints = layout.Exact(size)
	layout.Center.Layout(g, func(gtx C) D {
		w := min(gtx.Constraints.Max.X-gtx.Dp(48), gtx.Dp(380))
		gtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return mark.Paint(gtx, 30, kit.Fade(th.TextFaint, 0.55)) }),
			layout.Rigid(kit.Gap(28)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeLarge, Weight: font.Medium, Color: titleColor, MaxLines: 2}, title)
			}),
			layout.Rigid(kit.Gap(22)),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return steps(gtx)
			}),
			layout.Rigid(kit.Gap(12)),
			layout.Rigid(extra),
		)
	})
}
