package icons

import (
	"image/color"

	"gioui.org/op"
	"gioui.org/op/clip"
)

type brandEntry struct {
	color uint32
	path  string
}

type Brand struct {
	Icon  *Icon
	Color color.NRGBA
}

var brands = loadBrands()

func loadBrands() map[string]Brand {
	out := make(map[string]Brand, len(brandData))

	for id, b := range brandData {
		segs := parsePath(b.path)
		out[id] = Brand{
			Icon: outlined(func(ops *op.Ops, s float32) clip.PathSpec {
				return build(ops, segs, s/24)
			}),
			Color: color.NRGBA{R: uint8(b.color >> 16), G: uint8(b.color >> 8), B: uint8(b.color), A: 0xff},
		}
	}

	return out
}

func OS(id string) (Brand, bool) {
	b, ok := brands[id]
	return b, ok
}
