package kit

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/theme"
)

const testWindow = 600

func testKit() *Kit {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	return New(theme.New("carbon"), shaper)
}

func frame(r *input.Router, ops *op.Ops, w func(gtx C)) {
	ops.Reset()
	gtx := C{
		Ops:         ops,
		Source:      r.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(testWindow, testWindow)),
	}
	w(gtx)
	r.Frame(ops)
}

func press(r *input.Router, at f32.Point) {
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
}

func TestFieldFocusesFromAnywhereInTheBox(t *testing.T) {
	k := testKit()
	h := float32(k.Th.FieldHeight)
	points := map[string]f32.Point{
		"left padding":  f32.Pt(2, h/2),
		"past the text": f32.Pt(testWindow-40, h/2),
		"top edge":      f32.Pt(testWindow/2, 1),
		"bottom edge":   f32.Pt(testWindow/2, h-1),
	}

	for name, at := range points {
		t.Run(name, func(t *testing.T) {
			var r input.Router
			ops := new(op.Ops)
			ed := &widget.Editor{SingleLine: true}
			focused := false
			draw := func(gtx C) {
				k.Field(gtx, ed, FieldOpts{Hint: "hint"})
				focused = gtx.Focused(ed)
			}

			frame(&r, ops, draw)
			press(&r, at)

			for range 3 {
				frame(&r, ops, draw)
			}

			if !focused {
				t.Fatal("the field didn't take the focus")
			}
		})
	}
}

func TestDialogClosesOnPressOutsideTheCard(t *testing.T) {
	cases := []struct {
		name string
		at   f32.Point
		want bool
	}{
		{"outside", f32.Pt(5, 5), true},
		{"inside", f32.Pt(testWindow/2, testWindow/2), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := testKit()
			var r input.Router
			ops := new(op.Ops)
			d := &Dialog{}
			closed := false
			draw := func(gtx C) {
				closed = k.Dialog(gtx, d, DialogOpts{Title: "Title", Width: 300, Body: func(gtx C) D {
					return D{Size: image.Pt(gtx.Constraints.Max.X, 200)}
				}}) || closed
			}

			frame(&r, ops, draw)
			press(&r, c.at)

			for range 3 {
				frame(&r, ops, draw)
			}

			if closed != c.want {
				t.Fatalf("closed = %v, want %v", closed, c.want)
			}
		})
	}
}

func TestBordersSurviveClipping(t *testing.T) {
	const size = 40
	w, err := headless.NewWindow(size, size)

	if err != nil {
		t.Skip(err)
	}

	defer w.Release()
	k := testKit()
	ops := new(op.Ops)
	gtx := C{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(size, size))}
	bg := color.NRGBA{A: 255}
	paint.Fill(ops, bg)
	var b widget.Bool

	gtx.Constraints.Min = image.Point{}
	k.Checkbox(gtx, &b, "")

	if err := w.Frame(ops); err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))

	if err := w.Screenshot(img); err != nil {
		t.Fatal(err)
	}

	line := k.Th.FieldLine
	got := img.RGBAAt(0, 9)

	if diff := int(got.R) - int(line.R)*int(line.A)/255; diff < -2 || diff > 2 {
		t.Fatalf("left border pixel = %v, want the full border color %v", got, line)
	}
}
