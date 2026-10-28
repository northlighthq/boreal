package termview

import (
	"fmt"
	"image"
	"image/color"
	"slices"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/northlighthq/glyphs"
	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const (
	cursorBarWidth   = 2
	cursorLineHeight = 2
	scrollbarWidth   = 4
	faintAlpha       = 0x99
)

type filler func(r image.Rectangle, c color.NRGBA)

type rowEntry struct {
	cells []vt.Cell
	hash  uint64
	img   *image.RGBA
	op    paint.ImageOp
	valid bool
}

type rowCache struct {
	rows  []rowEntry
	size  image.Point
	stale bool
	dirty []int
	pool  []rowEntry
	taken []bool
}

func (c *rowCache) invalidate() {
	c.rows = nil
	c.stale = true
}

func hashCells(cells []vt.Cell) uint64 {
	h := uint64(14695981039346656037)

	for _, c := range cells {
		h = (h ^ (uint64(uint32(c.Rune)) | uint64(c.Fg)<<32)) * 1099511628211
		h = (h ^ (uint64(c.Bg) | uint64(c.Attr)<<32 | uint64(c.Link)<<48)) * 1099511628211
	}

	return h
}

func (v *View) paintRows(gtx layout.Context, fresh bool) {
	f := &v.frame
	c := &v.cache
	rowSize := image.Pt(f.Cols*v.m.w, v.m.h)

	if len(c.rows) != f.Rows || c.size != rowSize {
		c.rows = make([]rowEntry, f.Rows)
		c.size = rowSize
		c.stale = true
	}

	if fresh || c.stale {
		v.updateRows()
		c.stale = false
	}

	for y := range c.rows {
		off := op.Offset(image.Pt(0, y*rowSize.Y)).Push(gtx.Ops)
		area := clip.Rect{Max: rowSize}.Push(gtx.Ops)
		c.rows[y].op.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		area.Pop()
		off.Pop()
	}
}

func (v *View) updateRows() {
	f := &v.frame
	c := &v.cache

	if f.AltScreen != v.hlAlt {
		v.hlAlt = f.AltScreen
		c.invalidate()
	}

	c.dirty = c.dirty[:0]
	c.pool = c.pool[:0]

	for y := range c.rows {
		e := &c.rows[y]

		if !e.valid || !slices.Equal(e.cells, f.Row(y)) {
			c.dirty = append(c.dirty, y)
			c.pool = append(c.pool, *e)
		}
	}

	if len(c.dirty) == 0 {
		return
	}

	c.taken = slices.Grow(c.taken[:0], len(c.pool))[:len(c.pool)]
	clear(c.taken)
	todo := c.dirty[:0]

	for _, y := range c.dirty {
		cells := f.Row(y)
		h := hashCells(cells)
		found := -1

		for i, e := range c.pool {
			if !c.taken[i] && e.valid && e.hash == h && slices.Equal(e.cells, cells) {
				found = i
				break
			}
		}

		if found < 0 {
			todo = append(todo, y)
			continue
		}

		c.taken[found] = true
		c.rows[y] = c.pool[found]
	}

	next := 0

	for _, y := range todo {
		for c.taken[next] {
			next++
		}

		e := c.pool[next]
		c.taken[next] = true
		cells := f.Row(y)

		if e.img == nil || e.img.Bounds().Size() != c.size {
			e.img = image.NewRGBA(image.Rectangle{Max: c.size})
		}

		v.rasterRow(e.img, cells)
		e.cells = append(e.cells[:0], cells...)
		e.hash = hashCells(cells)
		e.op = paint.NewImageOp(e.img)
		e.op.Filter = paint.FilterNearest
		e.valid = true
		c.rows[y] = e
	}
}

func (v *View) rasterRow(img *image.RGBA, cells []vt.Cell) {
	cells = v.highlight(cells)
	b := img.Bounds()

	for y := b.Min.Y; y < b.Max.Y; y++ {
		i := img.PixOffset(b.Min.X, y)
		clear(img.Pix[i : i+b.Dx()*4])
	}

	origin := b.Min
	fill := func(r image.Rectangle, c color.NRGBA) { fillImage(img, r.Add(origin), c) }

	for x := 0; x < len(cells); {
		_, bg := v.colors(cells[x])
		start := x

		for x < len(cells) {
			if _, bc := v.colors(cells[x]); bc != bg {
				break
			}

			x++
		}

		if bg != v.theme.Background {
			fill(image.Rect(start*v.m.w, 0, x*v.m.w, v.m.h), bg)
		}
	}

	ligs := v.ligatures(cells)

	for x, c := range cells {
		if c.Width() == 0 || c.Rune == 0 || c.Rune == ' ' {
			continue
		}

		fg, _ := v.colors(c)

		if isBoxRune(c.Rune) {
			v.drawBox(fill, c.Rune, x*v.m.w, fg)
			continue
		}

		g := v.glyphs.Get(c.Rune, cellStyle(c), int(c.Width()))

		if ligs != nil && ligs[x] != 0 {
			g = v.glyphs.GetGlyph(ligs[x], cellStyle(c))
		}

		blitGlyph(img, g, origin.Add(image.Pt(x*v.m.w, 0)), fg)
	}

	v.paintDecorations(fill, cells)
}

func cellStyle(c vt.Cell) glyphs.Style {
	bold := c.Attr&vt.AttrBold != 0
	italic := c.Attr&vt.AttrItalic != 0

	switch {
	case bold && italic:
		return glyphs.BoldItalic
	case bold:
		return glyphs.Bold
	case italic:
		return glyphs.Italic
	}

	return glyphs.Regular
}

func blitGlyph(dst *image.RGBA, g *glyphs.Glyph, org image.Point, fg color.NRGBA) {
	if g.Empty() {
		return
	}

	b := dst.Bounds()
	at := org.Add(g.Off)

	if g.Color != nil {
		src := g.Color
		sb := src.Bounds()

		for y := 0; y < sb.Dy(); y++ {
			dy := at.Y + y

			if dy < b.Min.Y || dy >= b.Max.Y {
				continue
			}

			for x := 0; x < sb.Dx(); x++ {
				dx := at.X + x

				if dx < b.Min.X || dx >= b.Max.X {
					continue
				}

				si := src.PixOffset(x, y)
				sa := uint32(src.Pix[si+3])

				if sa == 0 {
					continue
				}

				di := dst.PixOffset(dx, dy)
				ia := 255 - sa
				dst.Pix[di] = uint8(uint32(src.Pix[si]) + uint32(dst.Pix[di])*ia/255)
				dst.Pix[di+1] = uint8(uint32(src.Pix[si+1]) + uint32(dst.Pix[di+1])*ia/255)
				dst.Pix[di+2] = uint8(uint32(src.Pix[si+2]) + uint32(dst.Pix[di+2])*ia/255)
				dst.Pix[di+3] = uint8(sa + uint32(dst.Pix[di+3])*ia/255)
			}
		}

		return
	}

	m := g.Mask
	mw, mh := m.Rect.Dx(), m.Rect.Dy()
	fa := uint32(fg.A)

	for y := 0; y < mh; y++ {
		dy := at.Y + y

		if dy < b.Min.Y || dy >= b.Max.Y {
			continue
		}

		row := m.Pix[y*m.Stride : y*m.Stride+mw]

		for x, cov := range row {
			if cov == 0 {
				continue
			}

			dx := at.X + x

			if dx < b.Min.X || dx >= b.Max.X {
				continue
			}

			a := uint32(cov) * fa / 255
			di := dst.PixOffset(dx, dy)
			ia := 255 - a
			dst.Pix[di] = uint8((uint32(fg.R)*a + uint32(dst.Pix[di])*ia) / 255)
			dst.Pix[di+1] = uint8((uint32(fg.G)*a + uint32(dst.Pix[di+1])*ia) / 255)
			dst.Pix[di+2] = uint8((uint32(fg.B)*a + uint32(dst.Pix[di+2])*ia) / 255)
			dst.Pix[di+3] = uint8(a + uint32(dst.Pix[di+3])*ia/255)
		}
	}
}

func fillImage(dst *image.RGBA, r image.Rectangle, c color.NRGBA) {
	r = r.Intersect(dst.Bounds())
	a := uint32(c.A)
	ia := 255 - a
	pr, pg, pb := uint32(c.R)*a/255, uint32(c.G)*a/255, uint32(c.B)*a/255

	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := dst.PixOffset(r.Min.X, y)

		for x := r.Min.X; x < r.Max.X; x++ {
			p := dst.Pix[i : i+4 : i+4]

			if a == 255 {
				p[0], p[1], p[2], p[3] = uint8(pr), uint8(pg), uint8(pb), 255
			} else {
				p[0] = uint8(pr + uint32(p[0])*ia/255)
				p[1] = uint8(pg + uint32(p[1])*ia/255)
				p[2] = uint8(pb + uint32(p[2])*ia/255)
				p[3] = uint8(a + uint32(p[3])*ia/255)
			}

			i += 4
		}
	}
}

func (v *View) colors(c vt.Cell) (fg, bg color.NRGBA) {
	fgc := c.Fg

	if c.Attr&vt.AttrBold != 0 && fgc.Kind() == vt.ColorIndexed && fgc.Index() < 8 {
		fgc = vt.Indexed(fgc.Index() + 8)
	}

	fg = v.theme.Color(fgc, true)
	bg = v.theme.Color(c.Bg, false)

	if c.Attr&vt.AttrInverse != 0 {
		fg, bg = bg, fg
	}

	if c.Attr&vt.AttrFaint != 0 {
		fg.A = faintAlpha
	}

	if c.Attr&vt.AttrInvisible != 0 {
		fg = bg
	}

	return fg, bg
}

func (v *View) paintDecorations(fill filler, cells []vt.Cell) {
	thick := max(v.m.h/16, 1)

	for x, c := range cells {
		under := c.Attr&(vt.AttrUnderline|vt.AttrDoubleUnderline|vt.AttrCurlyUnderline) != 0
		strike := c.Attr&vt.AttrStrike != 0

		if !under && !strike {
			continue
		}

		fg, _ := v.colors(c)
		x0, x1 := x*v.m.w, (x+1)*v.m.w
		y := min(v.m.baseline+thick+1, v.m.h-thick)

		switch {
		case c.Attr&vt.AttrCurlyUnderline != 0:
			for px := x0; px < x1; px++ {
				dy := 0

				if (px/2)%2 == 0 {
					dy = thick
				}

				fill(image.Rect(px, y+dy, px+1, y+dy+thick), fg)
			}
		case c.Attr&vt.AttrDoubleUnderline != 0:
			fill(image.Rect(x0, y, x1, y+thick), fg)
			fill(image.Rect(x0, min(y+2*thick, v.m.h-thick), x1, min(y+3*thick, v.m.h)), fg)
		case under:
			fill(image.Rect(x0, y, x1, y+thick), fg)
		}

		if strike {
			sy := v.m.h / 2
			fill(image.Rect(x0, sy, x1, sy+thick), fg)
		}
	}
}

func (v *View) cellRect(y, x0, x1 int) image.Rectangle {
	return image.Rect(x0*v.m.w, y*v.m.h, x1*v.m.w, (y+1)*v.m.h)
}

func (v *View) paintSelection(gtx layout.Context) {
	if !v.sel.active {
		return
	}

	a, b := v.sel.bounds()
	f := &v.frame

	for y := 0; y < f.Rows; y++ {
		id := f.TopLine + y

		if id < a.Line || id > b.Line {
			continue
		}

		x0, x1 := 0, f.Cols

		if id == a.Line {
			x0 = a.Col
		}

		if id == b.Line {
			x1 = min(b.Col+1, f.Cols)
		}

		if x1 > x0 {
			fillRect(gtx, v.cellRect(y, x0, x1), v.theme.TermSelect)
		}
	}
}

func (v *View) paintSearch(gtx layout.Context) {
	if !v.search.open || len(v.search.matches) == 0 {
		return
	}

	f := &v.frame
	top, bottom := f.TopLine, f.TopLine+f.Rows-1

	for i, m := range v.search.matches {
		if m.End.Line < top || m.Start.Line > bottom {
			continue
		}

		c := v.theme.SearchMatch

		if i == v.search.current {
			c = theme.WithAlpha(v.theme.Accent, 0xaa)
		}

		for id := max(m.Start.Line, top); id <= min(m.End.Line, bottom); id++ {
			x0, x1 := 0, f.Cols

			if id == m.Start.Line {
				x0 = m.Start.Col
			}

			if id == m.End.Line {
				x1 = m.End.Col + 1
			}

			fillRect(gtx, v.cellRect(id-top, x0, x1), c)
		}
	}
}

type cursorImage struct {
	key struct {
		cell vt.Cell
		w, h int
		bg   color.NRGBA
		fg   color.NRGBA
	}
	img *image.RGBA
	op  paint.ImageOp
}

func (v *View) paintCursor(gtx layout.Context) {
	cur := v.frame.Cursor

	if !cur.Visible || cur.Y >= v.frame.Rows || cur.X >= v.frame.Cols {
		return
	}

	grow := float32(1)

	if v.focused {
		grow = v.cursorAlpha()

		if grow <= 0.02 {
			return
		}
	}

	cell := v.frame.Row(cur.Y)[cur.X]
	w := v.m.w

	if cell.Width() == 2 {
		w *= 2
	}

	r := image.Rect(cur.X*v.m.w, cur.Y*v.m.h, cur.X*v.m.w+w, (cur.Y+1)*v.m.h)
	shrunk := r

	if grow < 1 {
		cy, half := (r.Min.Y+r.Max.Y)/2, int(float32(r.Dy())*grow/2+0.5)
		shrunk.Min.Y, shrunk.Max.Y = cy-half, cy+half
	}

	col := v.theme.Cursor

	if !v.focused {
		kit.Border(gtx, r, 2, 1, theme.WithAlpha(col, 0x99))
		return
	}

	shape := v.set.CursorStyle

	if cur.Custom {
		shape = cur.Style
	}

	switch shape {
	case vt.CursorBar:
		paint.FillShape(gtx.Ops, col, clip.UniformRRect(image.Rect(r.Min.X, max(shrunk.Min.Y, r.Min.Y+1), r.Min.X+cursorBarWidth, min(shrunk.Max.Y, r.Max.Y-1)), 1).Op(gtx.Ops))
	case vt.CursorUnderline:
		cx, half := (r.Min.X+r.Max.X)/2, int(float32(r.Dx())*grow/2+0.5)
		fillRect(gtx, image.Rect(cx-half, r.Max.Y-cursorLineHeight, cx+half, r.Max.Y), col)
	default:
		ci := &v.cursorImg
		k := ci.key
		k.cell, k.w, k.h, k.bg, k.fg = cell, w, v.m.h, col, v.theme.CursorText

		if ci.img == nil || k != ci.key {
			ci.key = k
			ci.img = image.NewRGBA(image.Rect(0, 0, w, v.m.h))
			fillImage(ci.img, ci.img.Bounds(), col)

			if cell.Rune != 0 && cell.Rune != ' ' {
				if isBoxRune(cell.Rune) {
					img := ci.img
					v.drawBox(func(r image.Rectangle, c color.NRGBA) { fillImage(img, r, c) }, cell.Rune, 0, v.theme.CursorText)
				} else {
					blitGlyph(ci.img, v.glyphs.Get(cell.Rune, cellStyle(cell), int(cell.Width())), image.Point{}, v.theme.CursorText)
				}
			}

			ci.op = paint.NewImageOp(ci.img)
			ci.op.Filter = paint.FilterNearest
		}

		t := op.Offset(r.Min).Push(gtx.Ops)
		area := clip.Rect{Min: image.Pt(0, shrunk.Min.Y-r.Min.Y), Max: image.Pt(r.Dx(), shrunk.Max.Y-r.Min.Y)}.Push(gtx.Ops)
		ci.op.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		area.Pop()
		t.Pop()
	}
}

func (v *View) paintScrollbar(gtx layout.Context, size image.Point) {
	f := &v.frame

	if f.Offset == 0 || f.Scrollback == 0 {
		return
	}

	total := f.Scrollback + f.Rows
	trackH := size.Y - 8
	thumbH := max(trackH*f.Rows/total, v.m.h*2)
	top := 4 + (trackH-thumbH)*(f.Scrollback-f.Offset)/f.Scrollback
	x := size.X - scrollbarWidth - 3

	c := theme.WithAlpha(v.theme.TextFaint, 0x99)
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rect(x, top, x+scrollbarWidth, top+thumbH), 2).Op(gtx.Ops))
}

func (v *View) paintDebug(gtx layout.Context, size image.Point) {
	msg := fmt.Sprintf("%d×%d  %.2fms", v.frame.Cols, v.frame.Rows, float64(v.lastFrame.Microseconds())/1000)

	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return v.kit.Label(gtx, kit.TextStyle{Color: v.theme.Accent, Mono: true}, msg)
	})
	call := rec.Stop()

	defer op.Offset(image.Pt(size.X-dims.Size.X-8, 4)).Push(gtx.Ops).Pop()
	fillRect(gtx, image.Rectangle{Max: dims.Size}, v.theme.Panel)
	call.Add(gtx.Ops)
}

func fillRect(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}
