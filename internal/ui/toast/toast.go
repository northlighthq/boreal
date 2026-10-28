package toast

import (
	"image"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	toastLife  = 5 * time.Second
	toastWidth = unit.Dp(340)
	maxToasts  = 4
)

type toastKind uint8

const (
	toastInfo toastKind = iota
	toastSuccess
	toastError
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

func (t *Stack) push(kind toastKind, title, message string) {
	t.pushAction(kind, title, message, "", nil)
}

func (t *Stack) pushAction(kind toastKind, title, message, label string, action func()) {
	t.mu.Lock()
	it := &toast{kind: kind, title: title, message: message, born: time.Now(), label: label, action: action}
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

func (t *Stack) Info(title, msg string) { t.push(toastInfo, title, msg) }

func (t *Stack) Success(title, msg string) { t.push(toastSuccess, title, msg) }

func (t *Stack) Error(title, msg string) { t.push(toastError, title, msg) }

func (t *Stack) SuccessAction(title, msg, label string, fn func()) {
	t.pushAction(toastSuccess, title, msg, label, fn)
}

func (t *Stack) ErrorAction(title, msg, label string, fn func()) {
	t.pushAction(toastError, title, msg, label, fn)
}

func (t *Stack) Layout(gtx layout.Context, k *kit.Kit) {
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
		dims := layout.UniformInset(12).Layout(g, func(gtx layout.Context) layout.Dimensions {
			ic, col := icons.Info, th.TextMuted

			switch it.kind {
			case toastSuccess:
				ic, col = icons.Check, th.Success
			case toastError:
				ic, col = icons.Error, th.Danger
			}

			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ic.Layout(gtx, 18, col) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return k.Label(gtx, kit.TextStyle{Weight: font.Medium}, it.title)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if it.message == "" {
								return layout.Dimensions{}
							}

							return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, Size: th.TextSizeSmall, MaxLines: 3}, it.message)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if it.action == nil {
								return layout.Dimensions{}
							}

							return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return k.Button(gtx, &it.act, kit.Secondary, nil, it.label)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return k.IconButtonSized(gtx, &it.close, icons.Close, false, 20, 14, th.TextMuted)
				}),
			)
		})
		call := rec.Stop()

		ap := it.appear.Animate(gtx, 1, kit.Slow)
		y -= dims.Size.Y
		x := size.X - w - gtx.Dp(16) + int(float32(gtx.Dp(40))*(1-ap))
		off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		opacity := paint.PushOpacity(gtx.Ops, ap)
		r := image.Rectangle{Max: dims.Size}
		radius := gtx.Dp(12)
		k.SurfaceRect(gtx, r, radius, th.Popup, true)
		call.Add(gtx.Ops)
		opacity.Pop()
		off.Pop()
		y -= gtx.Dp(8)
	}
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
