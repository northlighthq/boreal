package app

import (
	"image"
	"math"
	"strconv"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

type connecting struct {
	start     time.Time
	left      time.Time
	orbit     float64
	cancel    widget.Clickable
	cancelled bool
}

const (
	solPlaySize  = 96
	stageFadeIn  = 220 * time.Millisecond
	stageFadeOut = 320 * time.Millisecond
)

func easeOut(x float64) float32 {
	x = min(max(x, 0), 1)
	return float32(1 - math.Pow(1-x, 3))
}

func (a *App) paneOverlay(gtx C, p *Pane) {
	if p.asker != nil {
		if pr := a.prompts.find(p.asker); pr != nil {
			a.promptInline(gtx, pr)
			return
		}
	}

	cv, f := &p.connecting, &p.failure
	now := gtx.Now

	switch {
	case p.isConnecting():
		if cv.start.IsZero() {
			cv.start, cv.left = now, time.Time{}

			if !f.start.IsZero() {
				cv.start = now.Add(-stageFadeIn)
			}
		}

		f.start = time.Time{}
		t := now.Sub(cv.start).Seconds()
		a.connectingView(gtx, p, t, easeOut(t/stageFadeIn.Seconds()))
	case p.failed():
		if f.start.IsZero() {
			f.start = now

			if !cv.start.IsZero() {
				f.orbit = now.Sub(cv.start).Seconds()
			}
		}

		cv.start = time.Time{}
		a.failedView(gtx, p)
	default:
		if !cv.start.IsZero() {
			cv.left, cv.orbit, cv.start = now, now.Sub(cv.start).Seconds(), time.Time{}
		}

		f.start = time.Time{}

		if d := now.Sub(cv.left); !cv.left.IsZero() && d < stageFadeOut {
			a.connectingView(gtx, p, cv.orbit+d.Seconds(), 1-easeOut(d.Seconds()/stageFadeOut.Seconds()))
		}

		if p.lost() {
			a.lostBanner(gtx, p)
		} else {
			f.shown = time.Time{}
		}
	}
}

func layoutAt(gtx C, at image.Point) op.TransformStack {
	return op.Offset(at).Push(gtx.Ops)
}

func (a *App) connectingView(gtx C, p *Pane, t float64, opacity float32) {
	k := a.kit
	th := a.th()
	cv := &p.connecting

	if cv.cancel.Clicked(gtx) {
		cv.cancelled = true
	}

	who := sshx.User(p.sess.Target().Host) + "@" + p.sess.Target().Host.Address

	a.brandStage(gtx, opacity, true, "Connecting to "+who, th.Text,
		func(gtx C) D { return a.stepList(gtx, p, true) },
		func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return k.Button(gtx, &cv.cancel, kit.Ghost, nil, "Cancel") }),
				layout.Rigid(kit.Gap(10)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, strconv.Itoa(int(t))+" s")
				}),
			)
		})
	gtx.Execute(op.InvalidateCmd{})
}

func (p *Pane) isConnecting() bool {
	st, _ := p.sess.State()
	return p.sess.Kind == session.SSH && st == session.Connecting
}

func (a *App) promptInline(gtx C, pr *prompt) {
	th := a.th()
	k := a.kit
	size := gtx.Constraints.Max
	kit.Rect(gtx, image.Rectangle{Max: size}, th.Editor)

	if pr.handle(gtx) {
		return
	}

	o := pr.opts(gtx)
	g := gtx
	g.Constraints = layout.Exact(size)
	layout.Center.Layout(g, func(gtx C) D {
		w := min(gtx.Constraints.Max.X-gtx.Dp(48), gtx.Dp(o.Width))
		gtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeLarge + 2, Weight: font.SemiBold}, o.Title)
			}),
			layout.Rigid(kit.Gap(14)),
			layout.Rigid(o.Body),
			layout.Rigid(kit.Gap(22)),
			layout.Rigid(func(gtx C) D { return layout.E.Layout(gtx, o.Footer) }),
		)
	})
}
