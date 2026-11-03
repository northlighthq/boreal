package vt

type line struct {
	cells   []Cell
	wrapped bool
}

func newLine(cols int, bg Color) line {
	l := line{cells: make([]Cell, cols)}
	l.clear(0, cols, bg)
	return l
}

func (l *line) clear(from, to int, bg Color) {
	to = min(to, len(l.cells))

	for i := max(from, 0); i < to; i++ {
		l.cells[i] = blank(bg)
	}
}

func (l *line) reset(cols int, bg Color) {
	if cap(l.cells) < cols {
		l.cells = make([]Cell, cols)
	}

	l.cells = l.cells[:cols]
	l.wrapped = false
	l.clear(0, cols, bg)
}

func (l *line) resize(cols int) {
	old := len(l.cells)

	if cols <= old {
		l.cells = l.cells[:cols]
		return
	}

	if cap(l.cells) < cols {
		cells := make([]Cell, cols)
		copy(cells, l.cells)
		l.cells = cells
	}

	l.cells = l.cells[:cols]
	l.clear(old, cols, DefaultColor)
}

type buffer struct {
	lines []line
}

func newBuffer(cols, rows int) *buffer {
	b := &buffer{lines: make([]line, rows)}

	for i := range b.lines {
		b.lines[i] = newLine(cols, DefaultColor)
	}

	return b
}

type scrollback struct {
	lines   []line
	start   int
	size    int
	max     int
	dropped int
}

func (s *scrollback) len() int {
	return s.size
}

func (s *scrollback) at(i int) *line {
	return &s.lines[(s.start+i)%len(s.lines)]
}

func (s *scrollback) push(l line) (recycled line) {
	if s.max == 0 {
		s.dropped++
		return l
	}

	if s.size == len(s.lines) && len(s.lines) >= s.max {
		s.dropped++
		s.lines[s.start] = compactInto(s.lines[s.start].cells, l)
		s.start = (s.start + 1) % len(s.lines)
		return l
	}

	stored := compact(l)

	if s.size < len(s.lines) {
		s.lines[(s.start+s.size)%len(s.lines)] = stored
		s.size++
		return l
	}

	if len(s.lines) < s.max {
		s.lines = append(s.lines, stored)
		s.size++
		return l
	}

	s.dropped++
	s.lines[s.start] = stored
	s.start = (s.start + 1) % len(s.lines)
	return l
}

func compact(l line) line {
	return compactInto(nil, l)
}

func compactInto(buf []Cell, l line) line {
	n := trimmedLen(l)

	if cap(buf) < n {
		buf = make([]Cell, n)
	}

	buf = buf[:n]
	copy(buf, l.cells)
	return line{cells: buf, wrapped: l.wrapped}
}

func trimmedLen(l line) int {
	n := len(l.cells)

	if !l.wrapped {
		for n > 0 && emptyCell(l.cells[n-1]) {
			n--
		}
	}

	return n
}

func (s *scrollback) adopt(l line) {
	if s.max == 0 {
		s.dropped++
		return
	}

	n := trimmedLen(l)
	l.cells = l.cells[:n:n]

	if s.size < len(s.lines) {
		s.lines[(s.start+s.size)%len(s.lines)] = l
		s.size++
		return
	}

	if len(s.lines) < s.max {
		s.lines = append(s.lines, l)
		s.size++
		return
	}

	s.dropped++
	s.lines[s.start] = l
	s.start = (s.start + 1) % len(s.lines)
}

func emptyCell(c Cell) bool {
	return (c.Rune == 0 || c.Rune == ' ') && c.Width() == 1 && c.Style() == 0 && c.Bg == DefaultColor && c.Link == 0
}

func (s *scrollback) pop() (line, bool) {
	if s.size == 0 {
		return line{}, false
	}

	s.size--
	idx := (s.start + s.size) % len(s.lines)
	l := s.lines[idx]
	s.lines[idx] = line{}
	return l, true
}

func (s *scrollback) clear() {
	s.dropped += s.size
	s.lines = nil
	s.start = 0
	s.size = 0
}
