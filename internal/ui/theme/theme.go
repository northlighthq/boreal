package theme

import (
	"image/color"

	"gioui.org/unit"
	"github.com/plattyssh/vt"
)

const (
	Northlight = "northlight"
	Carbon     = "carbon"
	Graphite   = "graphite"
	Midnight   = "midnight"
	Paper      = "paper"
	Catppuccin = "catppuccin"
	RosePine   = "rosepine"
	Nord       = "nord"
	Everforest = "everforest"
	Gruvbox    = "gruvbox"
	TokyoNight = "tokyonight"
	Latte      = "latte"
	Dawn       = "dawn"
)

var Names = []string{Northlight, Carbon, Graphite, Midnight, Paper, Catppuccin, RosePine, Nord, Everforest, Gruvbox, TokyoNight, Latte, Dawn}

var Labels = []string{"Northlight", "Carbon", "Graphite", "Midnight", "Paper", "Catppuccin", "Rosé Pine", "Nord", "Everforest", "Gruvbox", "Tokyo Night", "Latte", "Rosé Pine Dawn"}

type Metrics struct {
	ToolbarHeight unit.Dp
	StatusHeight  unit.Dp
	TabHeight     unit.Dp
	RowHeight     unit.Dp
	FieldHeight   unit.Dp
	Radius        unit.Dp
	RadiusSmall   unit.Dp
	RadiusLarge   unit.Dp
	Gap           unit.Dp

	TextSize      unit.Sp
	TextSizeSmall unit.Sp
	TextSizeLarge unit.Sp
	TextSizeTitle unit.Sp
}

type Theme struct {
	Name string
	Dark bool
	Metrics

	Frame       color.NRGBA
	FrameText   color.NRGBA
	FrameMuted  color.NRGBA
	FramePanel  color.NRGBA
	FrameHover  color.NRGBA
	FrameBorder color.NRGBA

	Window    color.NRGBA
	Panel     color.NRGBA
	Editor    color.NRGBA
	Popup     color.NRGBA
	Field     color.NRGBA
	Border    color.NRGBA
	Divider   color.NRGBA
	FieldLine color.NRGBA
	Hover     color.NRGBA
	Pressed   color.NRGBA
	Selection color.NRGBA
	Inactive  color.NRGBA
	Scrim     color.NRGBA
	Dim       color.NRGBA

	Text      color.NRGBA
	TextMuted color.NRGBA
	TextFaint color.NRGBA
	OnAccent  color.NRGBA

	Accent      color.NRGBA
	AccentHover color.NRGBA
	AccentSoft  color.NRGBA
	Success     color.NRGBA
	Warning     color.NRGBA
	Danger      color.NRGBA

	Foreground  color.NRGBA
	Background  color.NRGBA
	Cursor      color.NRGBA
	CursorText  color.NRGBA
	TermSelect  color.NRGBA
	SearchMatch color.NRGBA
	ANSI        [16]color.NRGBA

	Tags []color.NRGBA

	palette [256]color.NRGBA
}

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func alpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

var metrics = Metrics{
	ToolbarHeight: 48,
	StatusHeight:  0,
	TabHeight:     34,
	RowHeight:     32,
	FieldHeight:   36,
	Radius:        8,
	RadiusSmall:   6,
	RadiusLarge:   12,
	Gap:           8,
	TextSize:      14,
	TextSizeSmall: 12,
	TextSizeLarge: 16,
	TextSizeTitle: 24,
}

type spec struct {
	name               string
	dark               bool
	canvas, surface    uint32
	raised, popup      uint32
	text, muted, faint uint32
	danger             uint32
	accent             uint32
	success, warning   uint32
	ansi               [16]uint32
}

var ansiDark = [16]uint32{
	0x1c1c1c, 0xf0605a, 0x8fd16a, 0xf2c45c,
	0x5fa8f5, 0xc58af0, 0x4fd1c5, 0xc8c8c8,
	0x5a5a5a, 0xff8a84, 0xaee68f, 0xffd98a,
	0x8cc2ff, 0xdbb0ff, 0x7fe6da, 0xf4f4f4,
}

var ansiLight = [16]uint32{
	0x1f1f1f, 0xc8352f, 0x3d8a26, 0x9a6a00,
	0x2563c9, 0x8a3fc4, 0x14857a, 0x8a8a8a,
	0x5c5c5c, 0xe0483f, 0x4ea332, 0xb88200,
	0x3b7be0, 0xa45ad9, 0x1f9e91, 0xa3a3a3,
}

var specs = map[string]spec{
	Northlight: {
		name: Northlight, dark: true,
		canvas: 0x0a0b0d, surface: 0x15171b, raised: 0x24272d, popup: 0x1b1e23,
		text: 0xf3f4f6, muted: 0x9a9fa8, faint: 0x60656e,
		danger: 0xe0625a, accent: 0x3bd39a, success: 0x3bd39a, warning: 0xf2c45c,
		ansi: ansiDark,
	},
	Carbon: {
		name: Carbon, dark: true,
		canvas: 0x0e0d0c, surface: 0x171615, raised: 0x25231f, popup: 0x1d1c1a,
		text: 0xece7e1, muted: 0x9b958d, faint: 0x67625b,
		danger: 0xe0625a, ansi: ansiDark,
	},
	Graphite: {
		name: Graphite, dark: true,
		canvas: 0x111111, surface: 0x1a1a1a, raised: 0x282828, popup: 0x202020,
		text: 0xf2f2f2, muted: 0x9a9a9a, faint: 0x666666,
		danger: 0xe0625a, ansi: ansiDark,
	},
	Midnight: {
		name: Midnight, dark: true,
		canvas: 0x0b0d12, surface: 0x12151c, raised: 0x1d212b, popup: 0x171a22,
		text: 0xe6e9ef, muted: 0x8b919c, faint: 0x575d68,
		danger: 0xe0625a, ansi: ansiDark,
	},
	Paper: {
		name: Paper, dark: false,
		canvas: 0xf1efeb, surface: 0xfdfcfa, raised: 0xe8e5df, popup: 0xffffff,
		text: 0x1c1a17, muted: 0x6f6a63, faint: 0xa39e96,
		danger: 0xc0392b, ansi: ansiLight,
	},
	Catppuccin: {
		name: Catppuccin, dark: true,
		canvas: 0x181825, surface: 0x1e1e2e, raised: 0x313244, popup: 0x252536,
		text: 0xcdd6f4, muted: 0xa6adc8, faint: 0x6c7086,
		danger: 0xf38ba8, accent: 0xcba6f7, success: 0xa6e3a1, warning: 0xf9e2af,
		ansi: [16]uint32{
			0x45475a, 0xf38ba8, 0xa6e3a1, 0xf9e2af, 0x89b4fa, 0xf5c2e7, 0x94e2d5, 0xbac2de,
			0x585b70, 0xf38ba8, 0xa6e3a1, 0xf9e2af, 0x89b4fa, 0xf5c2e7, 0x94e2d5, 0xa6adc8,
		},
	},
	RosePine: {
		name: RosePine, dark: true,
		canvas: 0x191724, surface: 0x1f1d2e, raised: 0x26233a, popup: 0x24223a,
		text: 0xe0def4, muted: 0x908caa, faint: 0x6e6a86,
		danger: 0xeb6f92, accent: 0xebbcba, success: 0x9ccfd8, warning: 0xf6c177,
		ansi: [16]uint32{
			0x26233a, 0xeb6f92, 0x31748f, 0xf6c177, 0x9ccfd8, 0xc4a7e7, 0xebbcba, 0xe0def4,
			0x6e6a86, 0xeb6f92, 0x31748f, 0xf6c177, 0x9ccfd8, 0xc4a7e7, 0xebbcba, 0xe0def4,
		},
	},
	Nord: {
		name: Nord, dark: true,
		canvas: 0x242933, surface: 0x2e3440, raised: 0x3b4252, popup: 0x343a47,
		text: 0xeceff4, muted: 0xa3acbd, faint: 0x6b768a,
		danger: 0xbf616a, accent: 0x88c0d0, success: 0xa3be8c, warning: 0xebcb8b,
		ansi: [16]uint32{
			0x3b4252, 0xbf616a, 0xa3be8c, 0xebcb8b, 0x81a1c1, 0xb48ead, 0x88c0d0, 0xe5e9f0,
			0x4c566a, 0xbf616a, 0xa3be8c, 0xebcb8b, 0x81a1c1, 0xb48ead, 0x8fbcbb, 0xeceff4,
		},
	},
	Everforest: {
		name: Everforest, dark: true,
		canvas: 0x232a2e, surface: 0x2d353b, raised: 0x3d484d, popup: 0x343f44,
		text: 0xd3c6aa, muted: 0x9da9a0, faint: 0x7a8478,
		danger: 0xe67e80, accent: 0xa7c080, success: 0x83c092, warning: 0xdbbc7f,
		ansi: [16]uint32{
			0x475258, 0xe67e80, 0xa7c080, 0xdbbc7f, 0x7fbbb3, 0xd699b6, 0x83c092, 0xd3c6aa,
			0x5c6a72, 0xe67e80, 0xa7c080, 0xdbbc7f, 0x7fbbb3, 0xd699b6, 0x83c092, 0xd3c6aa,
		},
	},
	Gruvbox: {
		name: Gruvbox, dark: true,
		canvas: 0x1d2021, surface: 0x282828, raised: 0x3c3836, popup: 0x32302f,
		text: 0xebdbb2, muted: 0xa89984, faint: 0x7c6f64,
		danger: 0xfb4934, accent: 0xfe8019, success: 0xb8bb26, warning: 0xfabd2f,
		ansi: [16]uint32{
			0x282828, 0xcc241d, 0x98971a, 0xd79921, 0x458588, 0xb16286, 0x689d6a, 0xa89984,
			0x928374, 0xfb4934, 0xb8bb26, 0xfabd2f, 0x83a598, 0xd3869b, 0x8ec07c, 0xebdbb2,
		},
	},
	TokyoNight: {
		name: TokyoNight, dark: true,
		canvas: 0x16161e, surface: 0x1a1b26, raised: 0x292e42, popup: 0x1f2335,
		text: 0xc0caf5, muted: 0x9aa5ce, faint: 0x565f89,
		danger: 0xf7768e, accent: 0x7aa2f7, success: 0x9ece6a, warning: 0xe0af68,
		ansi: [16]uint32{
			0x15161e, 0xf7768e, 0x9ece6a, 0xe0af68, 0x7aa2f7, 0xbb9af7, 0x7dcfff, 0xa9b1d6,
			0x414868, 0xf7768e, 0x9ece6a, 0xe0af68, 0x7aa2f7, 0xbb9af7, 0x7dcfff, 0xc0caf5,
		},
	},
	Latte: {
		name: Latte, dark: false,
		canvas: 0xe6e9ef, surface: 0xeff1f5, raised: 0xdce0e8, popup: 0xf7f8fa,
		text: 0x4c4f69, muted: 0x6c6f85, faint: 0x9ca0b0,
		danger: 0xd20f39, accent: 0x8839ef, success: 0x40a02b, warning: 0xdf8e1d,
		ansi: [16]uint32{
			0x5c5f77, 0xd20f39, 0x40a02b, 0xdf8e1d, 0x1e66f5, 0xea76cb, 0x179299, 0xacb0be,
			0x6c6f85, 0xd20f39, 0x40a02b, 0xdf8e1d, 0x1e66f5, 0xea76cb, 0x179299, 0xbcc0cc,
		},
	},
	Dawn: {
		name: Dawn, dark: false,
		canvas: 0xf2e9e1, surface: 0xfaf4ed, raised: 0xebe0d6, popup: 0xfffaf3,
		text: 0x575279, muted: 0x797593, faint: 0x9893a5,
		danger: 0xb4637a, accent: 0xd7827e, success: 0x56949f, warning: 0xea9d34,
		ansi: [16]uint32{
			0xf2e9e1, 0xb4637a, 0x286983, 0xea9d34, 0x56949f, 0x907aa9, 0xd7827e, 0x575279,
			0x9893a5, 0xb4637a, 0x286983, 0xea9d34, 0x56949f, 0x907aa9, 0xd7827e, 0x575279,
		},
	},
}

func New(name string) *Theme {
	return Build(name, Options{})
}

func build(sp spec) *Theme {

	surface, text := hex(sp.surface), hex(sp.text)
	fg := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	if !sp.dark {
		fg = color.NRGBA{A: 0xff}
	}

	line := func(f float32) color.NRGBA { return Mix(surface, fg, f) }

	t := &Theme{
		Name:    sp.name,
		Dark:    sp.dark,
		Metrics: metrics,

		Window:    hex(sp.canvas),
		Panel:     surface,
		Editor:    surface,
		Popup:     hex(sp.popup),
		Field:     surface,
		Border:    line(0.11),
		Divider:   line(0.11),
		FieldLine: line(0.14),
		Hover:     line(0.04),
		Pressed:   line(0.08),
		Selection: hex(sp.raised),
		Inactive:  hex(sp.raised),
		Scrim:     alpha(hex(0x000000), 0x99),
		Dim:       alpha(surface, 0x66),

		Text:      text,
		TextMuted: hex(sp.muted),
		TextFaint: hex(sp.faint),
		OnAccent:  surface,

		Accent:      text,
		AccentHover: fg,
		AccentSoft:  alpha(fg, 0x14),
		Danger:      hex(sp.danger),

		Foreground:  text,
		Background:  surface,
		Cursor:      text,
		CursorText:  surface,
		TermSelect:  line(0.22),
		SearchMatch: alpha(fg, 0x33),
	}
	t.Success = t.Text
	t.Warning = t.TextMuted

	if sp.accent != 0 {
		accent := hex(sp.accent)
		t.Accent, t.AccentHover, t.AccentSoft = accent, Mix(accent, fg, 0.2), alpha(accent, 0x26)
		t.Success, t.Warning = hex(sp.success), hex(sp.warning)
		t.Cursor, t.SearchMatch = accent, alpha(accent, 0x55)
		t.TermSelect = Mix(surface, accent, 0.25)

		if sp.dark {
			t.OnAccent = hex(sp.canvas)
		}

		for _, i := range []int{12, 10, 11, 9, 13, 14} {
			t.Tags = append(t.Tags, hex(sp.ansi[i]))
		}
	}

	if !sp.dark {
		t.Scrim = alpha(hex(0x000000), 0x33)
		t.Dim = alpha(surface, 0x55)
	}

	for i, v := range sp.ansi {
		t.ANSI[i] = hex(v)
	}

	canvas := hex(sp.canvas)
	t.Frame, t.FrameText, t.FrameMuted = canvas, t.Text, t.TextMuted
	t.FramePanel, t.FrameHover, t.FrameBorder = Mix(canvas, fg, 0.06), Mix(canvas, fg, 0.05), Mix(canvas, fg, 0.07)

	t.buildPalette()
	return t
}

func (t *Theme) Colorful() bool {
	return len(t.Tags) > 0
}

func (t *Theme) OnFrame() *Theme {
	return t
}

func (t *Theme) TagColor(i int) color.NRGBA {
	if len(t.Tags) == 0 {
		return t.TextMuted
	}

	return t.Tags[(i%len(t.Tags)+len(t.Tags))%len(t.Tags)]
}

func (t *Theme) buildPalette() {
	copy(t.palette[:16], t.ANSI[:])

	levels := [6]uint8{0, 95, 135, 175, 215, 255}

	for i := 0; i < 216; i++ {
		t.palette[16+i] = color.NRGBA{R: levels[i/36], G: levels[(i/6)%6], B: levels[i%6], A: 0xff}
	}

	for i := 0; i < 24; i++ {
		v := uint8(8 + i*10)
		t.palette[232+i] = color.NRGBA{R: v, G: v, B: v, A: 0xff}
	}
}

func (t *Theme) Color(c vt.Color, fg bool) color.NRGBA {
	switch c.Kind() {
	case vt.ColorIndexed:
		return t.palette[c.Index()]
	case vt.ColorRGB:
		r, g, b := c.RGB()
		return color.NRGBA{R: r, G: g, B: b, A: 0xff}
	}

	if fg {
		return t.Foreground
	}

	return t.Background
}

func Mix(a, b color.NRGBA, f float32) color.NRGBA {
	lerp := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*f) }
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: lerp(a.A, b.A)}
}

func WithAlpha(c color.NRGBA, a uint8) color.NRGBA {
	return alpha(c, a)
}
