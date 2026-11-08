package icons

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func renderIcon(t *testing.T, w *headless.Window, size int, draw func(gtx layout.Context)) *image.RGBA {
	t.Helper()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(size, size))}
	paint.Fill(ops, color.NRGBA{A: 255})

	for range 2 {
		draw(gtx)
	}

	if err := w.Frame(ops); err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))

	if err := w.Screenshot(img); err != nil {
		t.Fatal(err)
	}

	return img
}

func TestCachedIconsMatchDirectDrawing(t *testing.T) {
	const size = 32
	w, err := headless.NewWindow(size, size)

	if err != nil {
		t.Skip(err)
	}

	defer w.Release()
	colors := []color.NRGBA{{R: 230, G: 230, B: 240, A: 255}, {R: 255, G: 80, B: 80, A: 160}}
	tested := 0

	for _, ic := range []*Icon{Download, Upload, Settings, Close, Copy, Delete, Edit} {
		if ic.shape == nil {
			continue
		}

		tested++

		for _, c := range colors {
			px := size - 4
			direct := renderIcon(t, w, size, func(gtx layout.Context) {
				paint.FillShape(gtx.Ops, c, clip.Outline{Path: ic.shape(gtx.Ops, float32(px))}.Op())
			})
			cached := renderIcon(t, w, size, func(gtx layout.Context) {
				ic.Layout(gtx, unit.Dp(px), c)
			})

			for i := range direct.Pix {
				if direct.Pix[i] != cached.Pix[i] {
					t.Fatalf("pixel byte %d differs: %d vs %d", i, direct.Pix[i], cached.Pix[i])
				}
			}
		}
	}

	if tested == 0 {
		t.Fatal("no outlined icons were compared")
	}
}
