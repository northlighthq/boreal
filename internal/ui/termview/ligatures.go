package termview

import (
	"strings"

	"github.com/go-text/typesetting/font"
	"github.com/northlighthq/vt"
)

const ligatureChars = `-=<>!&|:/*.+~#%?^$@_[]{};w`

func ligatable(c vt.Cell) bool {
	return c.Width() == 1 && c.Rune > ' ' && c.Rune < 0x7f && !isBoxRune(c.Rune)
}

func (v *View) ligatures(cells []vt.Cell) []font.GID {
	if !v.set.Ligatures {
		return nil
	}

	var out []font.GID

	for x := 0; x < len(cells); {
		if !ligatable(cells[x]) {
			x++
			continue
		}

		start := x
		style := cellStyle(cells[x])
		fg, _ := v.colors(cells[x])
		candidate := false

		for x < len(cells) && ligatable(cells[x]) && cellStyle(cells[x]) == style {
			if c, _ := v.colors(cells[x]); c != fg {
				break
			}

			candidate = candidate || strings.ContainsRune(ligatureChars, cells[x].Rune)
			x++
		}

		if !candidate || x-start < 2 {
			continue
		}

		runes := make([]rune, x-start)

		for i := range runes {
			runes[i] = cells[start+i].Rune
		}

		gids := v.glyphs.Ligatures(runes, style)

		if gids == nil {
			continue
		}

		if out == nil {
			out = make([]font.GID, len(cells))
		}

		copy(out[start:], gids)
	}

	return out
}
