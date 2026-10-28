package app

import (
	"errors"
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/hosts"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

type failure struct {
	start  time.Time
	orbit  float64
	shown  time.Time
	retry  widget.Clickable
	edit   widget.Clickable
	banner widget.Clickable
}

func explain(err error, port int) (title, hint string) {
	if err == nil {
		return "Couldn't connect", "Something went wrong while opening the session."
	}

	msg := strings.ToLower(err.Error())
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(msg, p) {
				return true
			}
		}

		return false
	}

	switch {
	case errors.Is(err, sshx.ErrHostKeyRejected):
		return "Server key rejected", "The server's identity wasn't trusted, so Boreal didn't connect."
	case has("timeout", "timed out", "deadline exceeded"):
		return "The server didn't answer", "Check the address and that the server is online and reachable from this network."
	case has("connection refused"):
		return "Connection refused", "Nothing is accepting connections on port " + strconv.Itoa(port) + ". Is SSH running there?"
	case has("no such host", "server misbehaving", "name resolution"):
		return "Can't find this server", "The address didn't resolve. Check it for typos, or that you're on the right network or VPN."
	case has("network is unreachable", "no route to host", "host is unreachable"):
		return "Network unreachable", "This computer can't reach the server. Check your connection or VPN."
	case has("unable to authenticate", "permission denied", "auth"):
		return "Sign-in failed", "The server refused the credentials. Check the username, password or key."
	case has("reset by peer", "eof", "broken pipe"):
		return "The server closed the connection", "It hung up during the handshake. Try again in a moment."
	}

	return "Couldn't connect", "The connection failed before the session started."
}

func (p *Pane) failed() bool {
	st, _ := p.sess.State()
	return p.sess.Kind == session.SSH && st == session.Failed
}

func (p *Pane) lost() bool {
	st, _ := p.sess.State()
	return p.sess.Kind == session.SSH && st == session.Closed
}

func (a *App) failedView(gtx C, p *Pane) {
	k := a.kit
	th := a.th()
	f := &p.failure
	host := p.sess.Target().Host

	if f.retry.Clicked(gtx) {
		p.sess.Reconnect()
	}

	if f.edit.Clicked(gtx) && host.ID != "" {
		a.OpenDocked(core.Right, hosts.NewEditor(a, host))
	}

	_, err := p.sess.State()
	port := host.Port

	if port == 0 {
		port = 22
	}

	title, hint := explain(err, port)
	t := gtx.Now.Sub(f.start).Seconds()

	reveal := func(delay float64, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			x := easeOut(max(0, min(1, (t-delay)/0.45)))
			rec := op.Record(gtx.Ops)
			dims := w(gtx)
			call := rec.Stop()

			if x <= 0 {
				return dims
			}

			if x >= 1 {
				call.Add(gtx.Ops)
				return dims
			}
			off := op.Offset(image.Pt(0, int(float32(gtx.Dp(10))*(1-x)))).Push(gtx.Ops)
			opa := paint.PushOpacity(gtx.Ops, x)
			call.Add(gtx.Ops)
			opa.Pop()
			off.Pop()
			return dims
		})
	}

	a.brandStage(gtx, 1, false, title, th.Danger,
		func(gtx C) D { return a.stepList(gtx, p, false) },
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				reveal(0.5, func(gtx C) D {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(460))
					return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 3, Align: text.Middle}, hint)
				}),
				layout.Rigid(kit.Gap(22)),
				reveal(0.6, func(gtx C) D {
					children := []layout.FlexChild{
						layout.Rigid(func(gtx C) D { return k.Button(gtx, &f.retry, kit.Primary, icons.Refresh, "Try again") }),
					}

					if host.ID != "" {
						children = append(children, layout.Rigid(kit.Gap(6)),
							layout.Rigid(func(gtx C) D { return k.Button(gtx, &f.edit, kit.Ghost, nil, "Edit host") }))
					}

					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
				}),
			)
		})
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
}

func (a *App) lostBanner(gtx C, p *Pane) {
	k := a.kit
	th := a.th()
	f := &p.failure

	if f.banner.Clicked(gtx) {
		p.sess.Reconnect()
	}

	size := gtx.Constraints.Max
	rec := op.Record(gtx.Ops)
	cg := gtx
	cg.Constraints = layout.Constraints{Max: image.Pt(min(size.X-gtx.Dp(32), gtx.Dp(440)), size.Y)}
	dims := layout.Inset{Left: 14, Right: 8, Top: 8, Bottom: 8}.Layout(cg, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return kit.Dot(gtx, th.Danger, 8) }),
			layout.Rigid(kit.Gap(10)),
			layout.Flexed(1, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, MaxLines: 1}, "Connection lost")
			}),
			layout.Rigid(kit.Gap(12)),
			layout.Rigid(func(gtx C) D { return k.Button(gtx, &f.banner, kit.Primary, nil, "Reconnect") }),
		)
	})
	body := rec.Stop()

	if f.shown.IsZero() {
		f.shown = gtx.Now
	}

	x := easeOut(gtx.Now.Sub(f.shown).Seconds() / 0.35)

	if x < 1 {
		gtx.Execute(op.InvalidateCmd{})
	}

	at := image.Pt((size.X-dims.Size.X)/2, size.Y-dims.Size.Y-gtx.Dp(16)+int(float32(gtx.Dp(28))*(1-x)))
	off := op.Offset(at).Push(gtx.Ops)
	opa := paint.PushOpacity(gtx.Ops, x)
	k.SurfaceRect(gtx, image.Rectangle{Max: dims.Size}, dims.Size.Y/2, th.Popup, true)
	body.Add(gtx.Ops)
	opa.Pop()
	off.Pop()
}
