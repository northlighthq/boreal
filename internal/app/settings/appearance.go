package settings

import (
	"image"
	"strings"

	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

var tabStyles = []string{"classic", "pill", "underline"}

func index(list []string, v string) int {
	for i, s := range list {
		if strings.EqualFold(s, v) {
			return i
		}
	}

	return 0
}

func (d *Dialog) swatches(a core.Host, gtx C) D {
	k := a.Kit()
	th := k.Th
	base := theme.New(theme.Names[d.theme.Value]).Accent
	children := make([]layout.FlexChild, 0, len(theme.Accents)*2)

	for i, sw := range theme.Accents {
		if d.accents[i].Clicked(gtx) {
			d.accent = sw.Value
		}

		c := base

		if sw.Value != "" {
			c = theme.Build(theme.Names[d.theme.Value], theme.Options{Accent: sw.Value}).Accent
		}

		on := d.accent == sw.Value
		children = append(children, layout.Rigid(func(gtx C) D {
			return d.accents[i].Layout(gtx, func(gtx C) D {
				s := gtx.Dp(24)
				size := image.Pt(s, s)
				hv := k.Hover(gtx, kit.Key(&d.accents[i], 0), d.accents[i].Hovered())
				sel := k.Animated(gtx, kit.Key(&d.accents[i], 1), kit.B2F(on), kit.Normal)

				if sel > 0 {
					kit.RRect(gtx, image.Rectangle{Max: size}, s/2, kit.Fade(th.Text, sel))
					in := gtx.Dp(2)
					kit.RRect(gtx, image.Rect(in, in, s-in, s-in), s/2-in, th.Popup)
				}

				in := int(float32(gtx.Dp(4))*sel + float32(gtx.Dp(2))*(1-sel) - float32(gtx.Dp(1))*hv)
				kit.RRect(gtx, image.Rect(in, in, s-in, s-in), s/2-in, c)
				kit.Cursor(gtx, size, pointer.CursorPointer)
				return D{Size: size}
			})
		}), layout.Rigid(kit.Gap(6)))
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}
