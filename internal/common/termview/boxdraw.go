package termview

import (
	"image"
	"image/color"
)

const (
	weightNone   = 0
	weightLight  = 1
	weightHeavy  = 2
	weightDouble = 3
)

var boxLines = map[rune][4]uint8{
	'─': {0, 1, 0, 1}, '━': {0, 2, 0, 2}, '│': {1, 0, 1, 0}, '┃': {2, 0, 2, 0},
	'┄': {0, 1, 0, 1}, '┅': {0, 2, 0, 2}, '┆': {1, 0, 1, 0}, '┇': {2, 0, 2, 0},
	'┈': {0, 1, 0, 1}, '┉': {0, 2, 0, 2}, '┊': {1, 0, 1, 0}, '┋': {2, 0, 2, 0},
	'┌': {0, 1, 1, 0}, '┍': {0, 2, 1, 0}, '┎': {0, 1, 2, 0}, '┏': {0, 2, 2, 0},
	'┐': {0, 0, 1, 1}, '┑': {0, 0, 1, 2}, '┒': {0, 0, 2, 1}, '┓': {0, 0, 2, 2},
	'└': {1, 1, 0, 0}, '┕': {1, 2, 0, 0}, '┖': {2, 1, 0, 0}, '┗': {2, 2, 0, 0},
	'┘': {1, 0, 0, 1}, '┙': {1, 0, 0, 2}, '┚': {2, 0, 0, 1}, '┛': {2, 0, 0, 2},
	'├': {1, 1, 1, 0}, '┝': {1, 2, 1, 0}, '┞': {2, 1, 1, 0}, '┟': {1, 1, 2, 0},
	'┠': {2, 1, 2, 0}, '┡': {2, 2, 1, 0}, '┢': {1, 2, 2, 0}, '┣': {2, 2, 2, 0},
	'┤': {1, 0, 1, 1}, '┥': {1, 0, 1, 2}, '┦': {2, 0, 1, 1}, '┧': {1, 0, 2, 1},
	'┨': {2, 0, 2, 1}, '┩': {2, 0, 1, 2}, '┪': {1, 0, 2, 2}, '┫': {2, 0, 2, 2},
	'┬': {0, 1, 1, 1}, '┭': {0, 1, 1, 2}, '┮': {0, 2, 1, 1}, '┯': {0, 2, 1, 2},
	'┰': {0, 1, 2, 1}, '┱': {0, 1, 2, 2}, '┲': {0, 2, 2, 1}, '┳': {0, 2, 2, 2},
	'┴': {1, 1, 0, 1}, '┵': {1, 1, 0, 2}, '┶': {1, 2, 0, 1}, '┷': {1, 2, 0, 2},
	'┸': {2, 1, 0, 1}, '┹': {2, 1, 0, 2}, '┺': {2, 2, 0, 1}, '┻': {2, 2, 0, 2},
	'┼': {1, 1, 1, 1}, '┽': {1, 1, 1, 2}, '┾': {1, 2, 1, 1}, '┿': {1, 2, 1, 2},
	'╀': {2, 1, 1, 1}, '╁': {1, 1, 2, 1}, '╂': {2, 1, 2, 1}, '╃': {2, 1, 1, 2},
	'╄': {2, 2, 1, 1}, '╅': {1, 1, 2, 2}, '╆': {1, 2, 2, 1}, '╇': {2, 2, 1, 2},
	'╈': {1, 2, 2, 2}, '╉': {2, 1, 2, 2}, '╊': {2, 2, 2, 1}, '╋': {2, 2, 2, 2},
	'╌': {0, 1, 0, 1}, '╍': {0, 2, 0, 2}, '╎': {1, 0, 1, 0}, '╏': {2, 0, 2, 0},
	'═': {0, 3, 0, 3}, '║': {3, 0, 3, 0},
	'╒': {0, 3, 1, 0}, '╓': {0, 1, 3, 0}, '╔': {0, 3, 3, 0},
	'╕': {0, 0, 1, 3}, '╖': {0, 0, 3, 1}, '╗': {0, 0, 3, 3},
	'╘': {1, 3, 0, 0}, '╙': {3, 1, 0, 0}, '╚': {3, 3, 0, 0},
	'╛': {1, 0, 0, 3}, '╜': {3, 0, 0, 1}, '╝': {3, 0, 0, 3},
	'╞': {1, 3, 1, 0}, '╟': {3, 1, 3, 0}, '╠': {3, 3, 3, 0},
	'╡': {1, 0, 1, 3}, '╢': {3, 0, 3, 1}, '╣': {3, 0, 3, 3},
	'╤': {0, 3, 1, 3}, '╥': {0, 1, 3, 1}, '╦': {0, 3, 3, 3},
	'╧': {1, 3, 0, 3}, '╨': {3, 1, 0, 1}, '╩': {3, 3, 0, 3},
	'╪': {1, 3, 1, 3}, '╫': {3, 1, 3, 1}, '╬': {3, 3, 3, 3},
	'╭': {0, 1, 1, 0}, '╮': {0, 0, 1, 1}, '╯': {1, 0, 0, 1}, '╰': {1, 1, 0, 0},
	'╴': {0, 0, 0, 1}, '╵': {1, 0, 0, 0}, '╶': {0, 1, 0, 0}, '╷': {0, 0, 1, 0},
	'╸': {0, 0, 0, 2}, '╹': {2, 0, 0, 0}, '╺': {0, 2, 0, 0}, '╻': {0, 0, 2, 0},
	'╼': {0, 2, 0, 1}, '╽': {1, 0, 2, 0}, '╾': {0, 1, 0, 2}, '╿': {2, 0, 1, 0},
}

const (
	quadUL = 1 << iota
	quadUR
	quadLL
	quadLR
)

var quadrants = map[rune]uint8{
	'▖': quadLL, '▗': quadLR, '▘': quadUL, '▙': quadUL | quadLL | quadLR,
	'▚': quadUL | quadLR, '▛': quadUL | quadUR | quadLL, '▜': quadUL | quadUR | quadLR,
	'▝': quadUR, '▞': quadUR | quadLL, '▟': quadUR | quadLL | quadLR,
}

var shades = map[rune]uint8{'░': 0x40, '▒': 0x80, '▓': 0xc0}

func isBoxRune(r rune) bool {
	if r < 0x2500 || r > 0x259f {
		return false
	}

	if _, ok := boxLines[r]; ok {
		return true
	}

	return r >= 0x2580
}

func (v *View) drawBox(fill filler, r rune, x0 int, c color.NRGBA) {
	w, h := v.m.w, v.m.h

	if arms, ok := boxLines[r]; ok {
		v.drawLines(fill, arms, x0, c)
		return
	}

	if q, ok := quadrants[r]; ok {
		hw, hh := w/2, h/2

		if q&quadUL != 0 {
			fill(image.Rect(x0, 0, x0+hw, hh), c)
		}

		if q&quadUR != 0 {
			fill(image.Rect(x0+hw, 0, x0+w, hh), c)
		}

		if q&quadLL != 0 {
			fill(image.Rect(x0, hh, x0+hw, h), c)
		}

		if q&quadLR != 0 {
			fill(image.Rect(x0+hw, hh, x0+w, h), c)
		}

		return
	}

	if a, ok := shades[r]; ok {
		c.A = uint8(uint16(c.A) * uint16(a) / 0xff)
		fill(image.Rect(x0, 0, x0+w, h), c)
		return
	}

	switch {
	case r == '▀':
		fill(image.Rect(x0, 0, x0+w, h/2), c)
	case r >= '▁' && r <= '█':
		eighths := int(r-'▁') + 1
		fill(image.Rect(x0, h-h*eighths/8, x0+w, h), c)
	case r >= '▉' && r <= '▏':
		eighths := 8 - int(r-'▉') - 1
		fill(image.Rect(x0, 0, x0+w*eighths/8, h), c)
	case r == '▐':
		fill(image.Rect(x0+w/2, 0, x0+w, h), c)
	case r == '▔':
		fill(image.Rect(x0, 0, x0+w, max(h/8, 1)), c)
	case r == '▕':
		fill(image.Rect(x0+w-max(w/8, 1), 0, x0+w, h), c)
	}
}

func (v *View) drawLines(fill filler, arms [4]uint8, x0 int, c color.NRGBA) {
	w, h := v.m.w, v.m.h
	light := max((w+4)/8, 1)
	heavy := light * 2
	gap := light

	thick := func(weight uint8) int {
		if weight == weightHeavy {
			return heavy
		}

		return light
	}

	vExt, hExt := 0, 0

	for i, a := range arms {
		t := thick(a) / 2

		if a == weightDouble {
			t = gap + light
		}

		if a == weightNone {
			continue
		}

		if i%2 == 0 {
			hExt = max(hExt, t)
		} else {
			vExt = max(vExt, t)
		}
	}

	cx, cy := x0+w/2, h/2
	up, right, down, left := arms[0], arms[1], arms[2], arms[3]

	vertical := func(weight uint8, y0, y1 int) {
		if weight == weightDouble {
			fill(image.Rect(cx-gap-light, y0, cx-gap, y1), c)
			fill(image.Rect(cx+gap, y0, cx+gap+light, y1), c)
			return
		}

		t := thick(weight)
		fill(image.Rect(cx-t/2, y0, cx-t/2+t, y1), c)
	}

	horizontal := func(weight uint8, xa, xb int) {
		if weight == weightDouble {
			fill(image.Rect(xa, cy-gap-light, xb, cy-gap), c)
			fill(image.Rect(xa, cy+gap, xb, cy+gap+light), c)
			return
		}

		t := thick(weight)
		fill(image.Rect(xa, cy-t/2, xb, cy-t/2+t), c)
	}

	if up != weightNone {
		vertical(up, 0, cy+vExt+1)
	}

	if down != weightNone {
		vertical(down, cy-vExt, h)
	}

	if left != weightNone {
		horizontal(left, x0, cx+hExt+1)
	}

	if right != weightNone {
		horizontal(right, cx-hExt, x0+w)
	}
}
