package toast

import (
	"image"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	toastLife  = 5 * time.Second
	toastWidth = unit.Dp(360)
	maxToasts  = 4
	keyAction  = 0xf0
)

type toastKind uint8

const (
	toastInfo toastKind = iota
	toastSuccess
	toastError
	toastNotice
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type toast struct {
	kind    toastKind
	title   string
	message string
	born    time.Time
	close   widget.Clickable
	act     widget.Clickable
	label   string
	action  func()
	appear  kit.Anim
}

type Stack struct {
	mu       sync.Mutex
	items    []*toast
	onChange func()
}

func (t *Stack) push(it *toast) {
	t.mu.Lock()
	it.born = time.Now()
	it.appear.Set(0)
	t.items = append(t.items, it)

	if len(t.items) > maxToasts {
		t.items = t.items[len(t.items)-maxToasts:]
	}

	changed := t.onChange
	t.mu.Unlock()

	if changed != nil {
		changed()
	}
}

func (t *Stack) Info(title, msg string) { t.push(&toast{kind: toastInfo, title: title, message: msg}) }

func (t *Stack) Notice(title, msg, label string, fn func()) {
	t.push(&toast{kind: toastNotice, title: title, message: msg, label: label, action: fn})
}

func (t *Stack) Success(title, msg string) {
	t.push(&toast{kind: toastSuccess, title: title, message: msg})
}

func (t *Stack) Error(title, msg string) { t.push(&toast{kind: toastError, title: title, message: msg}) }

func (t *Stack) SuccessAction(title, msg, label string, fn func()) {
	t.push(&toast{kind: toastSuccess, title: title, message: msg, label: label, action: fn})
}

func (t *Stack) ErrorAction(title, msg, label string, fn func()) {
	t.push(&toast{kind: toastError, title: title, message: msg, label: label, action: fn})
}

func (t *Stack) Layout(gtx C, k *kit.Kit) {
	t.mu.Lock()
	now := time.Now()
	var live []*toast

	for _, it := range t.items {
		if it.act.Clicked(gtx) {
			go it.action()
			continue
		}

		if now.Sub(it.born) < it.life() && !it.close.Clicked(gtx) {
			live = append(live, it)
		}
	}

	t.items = live
	items := append([]*toast(nil), live...)
	t.mu.Unlock()

	if len(items) == 0 {
		return
	}

	next := items[0].born.Add(items[0].life())

	for _, it := range items[1:] {
		if at := it.born.Add(it.life()); at.Before(next) {
			next = at
		}
	}

	gtx.Execute(op.InvalidateCmd{At: next})

	th := k.Th
	size := gtx.Constraints.Max
	w := gtx.Dp(toastWidth)
	y := size.Y - gtx.Dp(th.StatusHeight) - gtx.Dp(12)

	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]

		rec := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, size.Y)}
		dims := layout.Inset{Top: 10, Bottom: 12, Left: 12, Right: 8}.Layout(g, func(gtx C) D {
			return it.layout(gtx, k)
		})
		call := rec.Stop()

		ap := it.appear.Animate(gtx, 1, kit.Slow)
		y -= dims.Size.Y
		x := size.X - w - gtx.Dp(16) + int(float32(gtx.Dp(40))*(1-ap))
		off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		opacity := paint.PushOpacity(gtx.Ops, ap)
		k.SurfaceRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(12), th.Popup, true)
		call.Add(gtx.Ops)
		opacity.Pop()
		off.Pop()
		y -= gtx.Dp(8)
	}
}

func (it *toast) layout(gtx C, k *kit.Kit) D {
	th := k.Th
	ic, col := icons.Info, th.TextMuted

	switch it.kind {
	case toastSuccess:
		ic, col = icons.Check, th.Success
	case toastError:
		ic, col = icons.Error, th.Danger
	case toastNotice:
		ic, col = icons.LucideBell, th.Accent
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 16, col) }),
				layout.Rigid(kit.Gap(8)),
				layout.Flexed(1, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Weight: font.Medium, MaxLines: 1}, it.title)
				}),
				layout.Rigid(func(gtx C) D {
					if it.action == nil {
						return D{}
					}

					return layout.Inset{Left: 12, Right: 4}.Layout(gtx, func(gtx C) D { return it.actionLink(gtx, k) })
				}),
				layout.Rigid(func(gtx C) D {
					return k.IconButtonSized(gtx, &it.close, icons.Close, false, 22, 14, th.TextMuted)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			if it.message == "" {
				return D{}
			}

			if it.kind == toastNotice {
				return layout.Inset{Top: 8, Right: 4}.Layout(gtx, func(gtx C) D { return it.line(gtx, k) })
			}

			return layout.Inset{Top: 2, Left: 24, Right: 26}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Size: th.TextSizeSmall, MaxLines: 3}, it.message)
			})
		}),
	)
}

func (it *toast) actionLink(gtx C, k *kit.Kit) D {
	th := k.Th
	h := k.Hover(gtx, kit.Key(&it.act, keyAction), it.act.Hovered())

	return it.act.Layout(gtx, func(gtx C) D {
		c := kit.Lerp(th.Accent, th.Text, h)
		dims := k.Label(gtx, kit.TextStyle{Color: c, Weight: font.Medium, Size: th.TextSizeSmall}, it.label)
		kit.Cursor(gtx, dims.Size, pointer.CursorPointer)
		return dims
	})
}

func (it *toast) line(gtx C, k *kit.Kit) D {
	th := k.Th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Top: 7, Bottom: 7, Left: 10, Right: 10}.Layout(gtx, func(gtx C) D {
		return k.Label(gtx, kit.TextStyle{Color: th.Foreground, Size: th.TextSizeSmall, MaxLines: 2, Mono: true}, it.message)
	})
	call := rec.Stop()

	r := image.Rectangle{Max: dims.Size}
	area := clip.UniformRRect(r, gtx.Dp(6)).Push(gtx.Ops)
	kit.Rect(gtx, r, th.Background)
	area.Pop()
	call.Add(gtx.Ops)
	return dims
}

func (t *Stack) OnChange(fn func()) {
	t.mu.Lock()
	t.onChange = fn
	t.mu.Unlock()
}

func (it *toast) life() time.Duration {
	if it.action != nil {
		return 2 * toastLife
	}

	return toastLife
}
