package vt

func (t *Terminal) reflow(cols, rows int, cur *cursor) {
	b := t.primary
	oldCols := t.cols

	last := cur.y

	for y := len(b.lines) - 1; y > last; y-- {
		if !lineEmpty(b.lines[y]) {
			last = y
			break
		}
	}

	sbLen := t.sb.len()
	phys := make([]line, 0, sbLen+last+1)

	for i := range sbLen {
		phys = append(phys, *t.sb.at(i))
	}

	curPhys := len(phys) + cur.y
	phys = append(phys, b.lines[:last+1]...)

	logicals := make([][]Cell, 0, len(phys))
	curLogical, curOffset := -1, 0
	var acc []Cell
	start := 0

	for i, l := range phys {
		cells := l.cells

		if len(cells) > oldCols {
			cells = cells[:oldCols]
		}

		if i == curPhys {
			curLogical = len(logicals)
			curOffset = cur.x

			for _, p := range phys[start:i] {
				curOffset += min(len(p.cells), oldCols)
			}
		}

		if l.wrapped && i != len(phys)-1 {
			continue
		}

		keep := -1

		if curLogical == len(logicals) {
			keep = curOffset
		}

		if start == i {
			logicals = append(logicals, trimBlankView(cells, keep))
		} else {
			acc = acc[:0]

			for _, p := range phys[start:i] {
				acc = append(acc, p.cells[:min(len(p.cells), oldCols)]...)
			}

			acc = append(acc, cells...)
			logicals = append(logicals, trimBlank(acc, keep))
		}

		start = i + 1
	}

	out := make([]line, 0, len(logicals))
	curLine, curX := 0, 0

	for li, cells := range logicals {
		first := len(out)
		var widths []int

		if len(cells) == 0 {
			out = append(out, line{})
			widths = append(widths, 0)
		}

		for len(cells) > 0 {
			n := min(cols, len(cells))

			if n < len(cells) && n > 1 && cells[n-1].Width() == 2 {
				n--
			}

			l := line{cells: cells[:n:n]}
			cells = cells[n:]
			l.wrapped = len(cells) > 0

			if l.wrapped && n < cols {
				l = padLine(l, cols)
			}

			out = append(out, l)

			if li == curLogical {
				widths = append(widths, n)
			}
		}

		if li != curLogical {
			continue
		}

		curLine, curX = first, curOffset

		for i, w := range widths {
			if curX < w || i == len(widths)-1 {
				break
			}

			curX -= w
			curLine++
		}

		switch {
		case curX == cols:
			curX = cols - 1
		case curX > cols:
			curLine++
			curX -= cols

			if curLine >= len(out) {
				out = append(out, line{})
			}
		}
	}

	startRow := max(len(out)-rows, 0)
	startRow = min(startRow, curLine)

	t.sb.clear()
	t.sb.lines = make([]line, 0, min(startRow, t.sb.max))

	for _, l := range out[:startRow] {
		t.sb.adopt(l)
	}

	screen := make([]line, rows)

	for i := range screen {
		screen[i] = newLine(cols, DefaultColor)

		if startRow+i < len(out) {
			copy(screen[i].cells, out[startRow+i].cells)
			screen[i].wrapped = out[startRow+i].wrapped
		}
	}

	b.lines = screen

	cur.y = min(curLine-startRow, rows-1)
	cur.x = min(curX, cols-1)
	cur.pendingWrap = false
}

func padLine(l line, cols int) line {
	padded := newLine(cols, DefaultColor)
	copy(padded.cells, l.cells)
	padded.wrapped = l.wrapped
	return padded
}

func lineEmpty(l line) bool {
	for _, c := range l.cells {
		if c.Rune != 0 || c.Bg != DefaultColor {
			return false
		}
	}

	return true
}

func trimBlank(cells []Cell, keep int) []Cell {
	view := trimBlankView(cells, keep)
	out := make([]Cell, len(view))
	copy(out, view)
	return out
}

func trimBlankView(cells []Cell, keep int) []Cell {
	n := len(cells)

	for n > 0 && cells[n-1].Rune == 0 && cells[n-1].Bg == DefaultColor && cells[n-1].Style() == 0 {
		n--
	}

	if keep >= 0 {
		n = max(n, min(keep, len(cells)))
	}

	return cells[:n:n]
}
