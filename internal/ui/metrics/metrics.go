package metrics

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/format"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const Height = unit.Dp(30)

type Bar struct {
	hover []widget.Clickable
}

func (b *Bar) Items() int {
	return len(b.hover)
}

func (b *Bar) Layout(gtx C, k *kit.Kit, s sshx.Stats, hist History, have bool, err error) D {
	th := k.Th
	h := gtx.Dp(Height)
	size := image.Pt(gtx.Constraints.Max.X, h)
	kit.Rect(gtx, image.Rectangle{Max: size}, kit.Lerp(th.Editor, th.Window, 0.45))
	muted := kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}

	var children []layout.FlexChild

	switch {
	case have:
		ms := serverMetrics(th, s, hist)

		if len(b.hover) < len(ms) {
			b.hover = make([]widget.Clickable, len(ms))
		}

		for i, m := range ms {
			children = append(children, layout.Rigid(func(gtx C) D { return b.item(gtx, k, &b.hover[i], m) }))
		}
	case err == sshx.ErrNoStats:
		children = append(children, layout.Rigid(func(gtx C) D { return k.Label(gtx, muted, "No metrics (not a Linux server)") }))
	default:
		children = append(children, layout.Rigid(func(gtx C) D { return k.Label(gtx, muted, "Reading metrics…") }))
	}

	gtx.Constraints = layout.Exact(size)
	layout.Inset{Left: 10, Right: 14}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = h
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})

	return D{Size: size}
}

type metric struct {
	icon   *icons.Icon
	frac   float64
	value  string
	title  string
	detail []string
	chart  []series
	top    float64
}

func serverMetrics(th *theme.Theme, s sshx.Stats, hist History) []metric {
	pct := func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) }
	size := func(n uint64) string { return format.Size(int64(n)) }

	cpu := metric{icon: icons.CPU, frac: max(s.CPU, 0), value: "…", title: "CPU"}

	if s.CPU >= 0 {
		cpu.value = pct(s.CPU)
	}

	cpu.chart, cpu.top = []series{{values: hist.CPU, color: level(th, cpu.frac), fill: true}}, 1

	cpu.detail = []string{
		fmt.Sprintf("%s in use · %d cores", cpu.value, s.Cores),
		fmt.Sprintf("Load %.2f / %.2f / %.2f (1, 5, 15 min)", s.Load1, s.Load5, s.Load15),
	}

	memFrac := ratio(s.MemUsed, s.MemTotal)
	mem := metric{icon: icons.Memory, frac: memFrac, value: size(s.MemUsed) + " / " + size(s.MemTotal), title: "Memory",
		detail: []string{
			fmt.Sprintf("%s used of %s (%s)", size(s.MemUsed), size(s.MemTotal), pct(memFrac)),
			size(s.MemTotal-s.MemUsed) + " available",
		}}

	mem.chart, mem.top = []series{{values: hist.Mem, color: level(th, memFrac), fill: true}}, 1

	if s.SwapTotal > 0 {
		mem.detail = append(mem.detail, fmt.Sprintf("Swap %s / %s", size(s.SwapUsed), size(s.SwapTotal)))
	}

	diskFrac := ratio(s.DiskUsed, s.DiskTotal)
	disk := metric{icon: icons.Disk, frac: diskFrac, value: pct(diskFrac), title: "Disk (/)",
		detail: []string{
			fmt.Sprintf("%s used of %s", size(s.DiskUsed), size(s.DiskTotal)),
			size(s.DiskAvail) + " free",
		}}

	disk.chart, disk.top = []series{{values: hist.Disk, color: level(th, diskFrac), fill: true}}, 1

	out := []metric{cpu, mem, disk}

	if s.RxRate >= 0 {
		out = append(out, metric{icon: icons.TransferOutline, frac: -1, title: "Network",
			value: "↓ " + format.Size(int64(s.RxRate)) + "/s  ↑ " + format.Size(int64(s.TxRate)) + "/s",
			detail: []string{
				"Received " + size(s.RxTotal) + " since boot",
				"Sent " + size(s.TxTotal) + " since boot",
			},
			chart: []series{
				{values: hist.Rx, color: netColor(th), fill: true},
				{values: hist.Tx, color: kit.Fade(th.TextMuted, 0.9)},
			},
			top: max(peak(hist.Rx, hist.Tx)*1.15, 1024),
		})
	}

	out = append(out,
		metric{icon: icons.Gauge, frac: -1, value: fmt.Sprintf("%.2f", s.Load1), title: "Load average",
			detail: []string{
				fmt.Sprintf("%.2f · %.2f · %.2f", s.Load1, s.Load5, s.Load15),
				"Last 1, 5 and 15 minutes",
			}},
		metric{icon: icons.Clock, frac: -1, value: format.Uptime(s.Uptime), title: "Uptime",
			detail: osLines(s)},
	)
	return out
}

func osLines(s sshx.Stats) []string {
	var l []string
	since := time.Now().Add(-s.Uptime)
	l = append(l, "Up since "+since.Format("Jan 2, 15:04"))

	if s.OS != "" {
		l = append(l, s.OS)
	}

	if s.Kernel != "" {
		l = append(l, s.Kernel)
	}

	if s.Hostname != "" {
		l = append(l, "Host "+s.Hostname)
	}

	return l
}

func (b *Bar) item(gtx C, k *kit.Kit, hover *widget.Clickable, m metric) D {
	th := k.Th
	value := kit.TextStyle{Size: th.TextSizeSmall, Color: th.Text}

	dims := hover.Layout(gtx, func(gtx C) D {
		hv := k.Hover(gtx, kit.Key(hover, 0), hover.Hovered())
		return layout.Inset{Left: 6, Right: 10}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(Height)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return m.icon.Layout(gtx, 14, kit.Lerp(iconColor(th, m), th.Text, hv)) }),
				layout.Rigid(kit.Gap(6)),
				layout.Rigid(func(gtx C) D {
					if m.frac < 0 {
						return D{}
					}

					return layout.Inset{Right: 6}.Layout(gtx, func(gtx C) D { return miniBar(gtx, k, m.frac) })
				}),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, value, m.value) }),
			)
		})
	})

	if hover.Hovered() {
		rec := op.Record(gtx.Ops)
		card := op.Record(gtx.Ops)
		cg := gtx
		cg.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(320), gtx.Dp(400))}
		cd := layout.UniformInset(10).Layout(cg, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Weight: font.SemiBold}, m.title)
				}),
				layout.Rigid(kit.Gap(4)),
			}

			if len(m.chart) > 0 && len(m.chart[0].values) >= 2 {
				rows = append(rows,
					layout.Rigid(kit.Gap(4)),
					layout.Rigid(func(gtx C) D {
						size := image.Pt(gtx.Dp(260), gtx.Dp(64))
						kit.RRect(gtx, image.Rectangle{Max: size}, gtx.Dp(th.RadiusSmall), kit.Fade(th.Text, 0.04))
						return chart(gtx, size, m.top, m.chart...)
					}),
					layout.Rigid(kit.Gap(8)),
				)
			}

			for _, l := range m.detail {
				rows = append(rows, layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, l)
				}))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
		body := card.Stop()
		off := op.Offset(image.Pt(0, -cd.Size.Y-gtx.Dp(6))).Push(gtx.Ops)
		k.SurfaceRect(gtx, image.Rectangle{Max: cd.Size}, gtx.Dp(th.RadiusSmall), th.Popup, true)
		body.Add(gtx.Ops)
		off.Pop()
		op.Defer(gtx.Ops, rec.Stop())
	}

	return dims
}

func miniBar(gtx C, k *kit.Kit, frac float64) D {
	th := k.Th
	w, h := gtx.Dp(38), gtx.Dp(5)
	r := image.Rect(0, 0, w, h)
	kit.RRect(gtx, r, h/2, kit.Fade(th.Text, 0.12))
	fill := level(th, frac)

	if fw := int(float64(w) * min(max(frac, 0), 1)); fw > 0 {
		kit.RRect(gtx, image.Rect(0, 0, max(fw, h), h), h/2, color.NRGBA(fill))
	}

	return D{Size: image.Pt(w, h)}
}

func ratio(a, b uint64) float64 {
	if b == 0 {
		return 0
	}

	return float64(a) / float64(b)
}
