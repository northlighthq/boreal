package theme

type Options struct {
	Accent   string
	Density  string
	Corners  string
	Terminal string
}

const (
	DensityCompact     = "compact"
	DensityNormal      = "normal"
	DensityComfortable = "comfortable"

	CornersSharp  = "sharp"
	CornersNormal = "normal"
	CornersRound  = "round"
)

var Densities = []string{DensityCompact, DensityNormal, DensityComfortable}

var CornerStyles = []string{CornersSharp, CornersNormal, CornersRound}

type Swatch struct {
	Name  string
	Value string
}

var Accents = []Swatch{
	{"Theme", ""},
	{"Blue", "blue"},
	{"Cyan", "cyan"},
	{"Green", "green"},
	{"Yellow", "yellow"},
	{"Red", "red"},
	{"Magenta", "magenta"},
}

var accentSlots = map[string]int{"red": 9, "green": 10, "yellow": 11, "blue": 12, "magenta": 13, "cyan": 14}

type scheme struct {
	name   string
	bg, fg uint32
	ansi   [16]uint32
}

var schemes = []scheme{
	{name: "Dracula", bg: 0x282a36, fg: 0xf8f8f2, ansi: [16]uint32{
		0x21222c, 0xff5555, 0x50fa7b, 0xf1fa8c, 0xbd93f9, 0xff79c6, 0x8be9fd, 0xf8f8f2,
		0x6272a4, 0xff6e6e, 0x69ff94, 0xffffa5, 0xd6acff, 0xff92df, 0xa4ffff, 0xffffff,
	}},
	{name: "Nord", bg: 0x2e3440, fg: 0xd8dee9, ansi: [16]uint32{
		0x3b4252, 0xbf616a, 0xa3be8c, 0xebcb8b, 0x81a1c1, 0xb48ead, 0x88c0d0, 0xe5e9f0,
		0x4c566a, 0xbf616a, 0xa3be8c, 0xebcb8b, 0x81a1c1, 0xb48ead, 0x8fbcbb, 0xeceff4,
	}},
	{name: "Gruvbox", bg: 0x282828, fg: 0xebdbb2, ansi: [16]uint32{
		0x282828, 0xcc241d, 0x98971a, 0xd79921, 0x458588, 0xb16286, 0x689d6a, 0xa89984,
		0x928374, 0xfb4934, 0xb8bb26, 0xfabd2f, 0x83a598, 0xd3869b, 0x8ec07c, 0xebdbb2,
	}},
	{name: "Catppuccin", bg: 0x1e1e2e, fg: 0xcdd6f4, ansi: [16]uint32{
		0x45475a, 0xf38ba8, 0xa6e3a1, 0xf9e2af, 0x89b4fa, 0xf5c2e7, 0x94e2d5, 0xbac2de,
		0x585b70, 0xf38ba8, 0xa6e3a1, 0xf9e2af, 0x89b4fa, 0xf5c2e7, 0x94e2d5, 0xa6adc8,
	}},
	{name: "Tokyo Night", bg: 0x1a1b26, fg: 0xc0caf5, ansi: [16]uint32{
		0x15161e, 0xf7768e, 0x9ece6a, 0xe0af68, 0x7aa2f7, 0xbb9af7, 0x7dcfff, 0xa9b1d6,
		0x414868, 0xf7768e, 0x9ece6a, 0xe0af68, 0x7aa2f7, 0xbb9af7, 0x7dcfff, 0xc0caf5,
	}},
	{name: "One Dark", bg: 0x282c34, fg: 0xabb2bf, ansi: [16]uint32{
		0x282c34, 0xe06c75, 0x98c379, 0xe5c07b, 0x61afef, 0xc678dd, 0x56b6c2, 0xabb2bf,
		0x5c6370, 0xe06c75, 0x98c379, 0xe5c07b, 0x61afef, 0xc678dd, 0x56b6c2, 0xffffff,
	}},
	{name: "Solarized", bg: 0x002b36, fg: 0x839496, ansi: [16]uint32{
		0x073642, 0xdc322f, 0x859900, 0xb58900, 0x268bd2, 0xd33682, 0x2aa198, 0xeee8d5,
		0x586e75, 0xcb4b16, 0x586e75, 0x657b83, 0x839496, 0x6c71c4, 0x93a1a1, 0xfdf6e3,
	}},
}

func Schemes() []string {
	out := []string{"Theme"}

	for _, s := range schemes {
		out = append(out, s.name)
	}

	return out
}

func Build(name string, o Options) *Theme {
	sp, ok := specs[name]

	if !ok {
		sp = specs[Northlight]
	}

	if slot, ok := accentSlots[o.Accent]; ok {
		sp.accent = sp.ansi[slot]

		if sp.success == 0 {
			sp.success, sp.warning = sp.ansi[10], sp.ansi[11]
		}
	}

	t := build(sp)
	t.Metrics = scaled(o.Density, o.Corners)

	for _, s := range schemes {
		if s.name != o.Terminal {
			continue
		}

		t.Background, t.Foreground = hex(s.bg), hex(s.fg)

		for i, v := range s.ansi {
			t.ANSI[i] = hex(v)
		}

		t.CursorText = t.Background
		t.TermSelect = Mix(t.Background, t.Cursor, 0.28)
		t.buildPalette()
	}

	return t
}

func scaled(density, corners string) Metrics {
	m := metrics

	switch density {
	case DensityCompact:
		m.RowHeight, m.FieldHeight, m.TabHeight = 28, 32, 30
		m.TextSize, m.TextSizeSmall, m.TextSizeLarge = 13, 11, 15
		m.Gap = 6
	case DensityComfortable:
		m.RowHeight, m.FieldHeight, m.TabHeight = 36, 40, 38
		m.TextSize, m.TextSizeSmall, m.TextSizeLarge = 15, 13, 17
		m.Gap = 10
	}

	switch corners {
	case CornersSharp:
		m.Radius, m.RadiusSmall, m.RadiusLarge = 3, 2, 4
	case CornersRound:
		m.Radius, m.RadiusSmall, m.RadiusLarge = 12, 9, 18
	}

	return m
}
