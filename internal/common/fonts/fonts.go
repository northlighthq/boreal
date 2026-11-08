package fonts

import (
	"embed"
	"os"
	"path/filepath"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"github.com/northlighthq/glyphs"
)

const (
	Mono font.Typeface = "Geist Mono"
	UI   font.Typeface = "Segoe UI, Geist"
)

//go:embed data/*.ttf
var data embed.FS

var monoFiles = []string{
	"GeistMono-Regular.ttf",
}

var uiFiles = []string{
	"Geist-Regular.ttf",
	"Geist-Medium.ttf",
	"Geist-SemiBold.ttf",
	"Geist-Bold.ttf",
}

func Load() (*text.Shaper, font.Font) {
	faces := embedded(monoFiles)

	if ui := systemUI(); len(ui) > 0 {
		faces = append(faces, ui...)
	} else {
		faces = append(faces, embedded(uiFiles)...)
	}

	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces)), font.Font{Typeface: Mono}
}

const (
	JetBrainsMono = "JetBrains Mono"
	FiraCode      = "Fira Code"
	GeistMono     = "Geist Mono"
)

var TerminalFonts = []string{JetBrainsMono, FiraCode, GeistMono}

var terminalFiles = map[string][4]string{
	JetBrainsMono: {"JetBrainsMono-Regular.ttf", "JetBrainsMono-Bold.ttf", "JetBrainsMono-Italic.ttf", "JetBrainsMono-BoldItalic.ttf"},
	FiraCode:      {"FiraCode-Regular.ttf", "FiraCode-Bold.ttf", "FiraCode-Regular.ttf", "FiraCode-Bold.ttf"},
	GeistMono:     {"GeistMono-Regular.ttf", "GeistMono-Bold.ttf", "GeistMono-Italic.ttf", "GeistMono-BoldItalic.ttf"},
}

var (
	glyphMu   sync.Mutex
	glyphSets = map[string]*glyphs.Set{}
)

func Glyphs(family string) (*glyphs.Set, error) {
	files, ok := terminalFiles[family]

	if !ok {
		family = TerminalFonts[0]
		files = terminalFiles[family]
	}

	glyphMu.Lock()
	defer glyphMu.Unlock()

	if s := glyphSets[family]; s != nil {
		return s, nil
	}

	var paths [4]string

	for i, f := range files {
		paths[i] = "data/" + f
	}

	s, err := glyphs.NewSet(data, paths)

	if err != nil {
		return nil, err
	}

	glyphSets[family] = s
	return s, nil
}

var systemUIFiles = []string{"segoeui.ttf", "seguisb.ttf"}

func systemUI() []font.FontFace {
	windir := os.Getenv("WINDIR")

	if windir == "" {
		return nil
	}

	dir := filepath.Join(windir, "Fonts")

	var faces []font.FontFace

	for _, name := range systemUIFiles {
		b, err := os.ReadFile(filepath.Join(dir, name))

		if err != nil {
			continue
		}

		fs, err := opentype.ParseCollection(b)

		if err != nil {
			continue
		}

		faces = append(faces, fs...)
	}

	return faces
}

func embedded(names []string) []font.FontFace {
	var faces []font.FontFace

	for _, name := range names {
		b, err := data.ReadFile("data/" + name)

		if err != nil {
			continue
		}

		fs, err := opentype.ParseCollection(b)

		if err != nil {
			continue
		}

		faces = append(faces, fs...)
	}

	return faces
}
