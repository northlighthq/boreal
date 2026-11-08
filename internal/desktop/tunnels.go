package desktop

import (
	"slices"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/format"
	"github.com/northlighthq/boreal/internal/core/sshx"
)

const trafficEvery = time.Second

type trafficRate struct {
	hostID string
	at     time.Time
	last   sshx.Traffic
	up     int64
	down   int64
}

func (a *App) autoTunnels(hostID string, c *sshx.Client) {
	if hostID == "" {
		return
	}

	dropped := a.tunnels.TakeDropped(hostID)

	for _, r := range a.vault.Snapshot().Forwards {
		switch {
		case r.HostID != hostID:
		case r.OnStart:
			a.tunnels.StartOn(r, c)
		case slices.Contains(dropped, r.ID):
			a.tunnels.Start(r)
		}
	}
}

func (a *App) tunnelTail(gtx C, hostID string) layout.Widget {
	n, t := a.tunnels.Traffic(hostID)

	if n == 0 {
		return nil
	}

	r := &a.rate

	if r.hostID != hostID || gtx.Now.Sub(r.at) > 3*trafficEvery {
		*r = trafficRate{hostID: hostID, at: gtx.Now, last: t}
	}

	if dt := gtx.Now.Sub(r.at); dt >= trafficEvery {
		r.up = int64(float64(max(t.Up-r.last.Up, 0)) / dt.Seconds())
		r.down = int64(float64(max(t.Down-r.last.Down, 0)) / dt.Seconds())
		r.at, r.last = gtx.Now, t
	}

	gtx.Execute(op.InvalidateCmd{At: r.at.Add(trafficEvery)})
	k := a.kit
	th := a.th()
	label := plural(n, "tunnel") + " · " + plural(t.Conns, "connection") + " · ↑ " + format.Size(r.up) + "/s ↓ " + format.Size(r.down) + "/s"

	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icons.ArrowsOutline.Layout(gtx, 14, th.TextMuted) }),
			layout.Rigid(kit.Gap(6)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 1}, label)
			}),
		)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}

	return strconv.Itoa(n) + " " + word + "s"
}
