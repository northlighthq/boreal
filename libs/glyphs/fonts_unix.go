//go:build !windows && !darwin && !android

package glyphs

import (
	"os"
	"path/filepath"
	"strings"
)

var fallbackFiles = []string{
	"NotoSansSymbols2-Regular.ttf",
	"NotoSansSymbols-Regular.ttf",
	"DejaVuSansMono.ttf",
	"NotoSansMono-Regular.ttf",
	"NotoSansCJK-Regular.ttc",
	"NotoSansCJKsc-Regular.otf",
	"wqy-microhei.ttc",
	"DejaVuSans.ttf",
	"NotoSans-Regular.ttf",
	"LiberationSans-Regular.ttf",
}

var emojiFiles = []string{"Noto-COLRv1.ttf", "NotoColorEmoji-COLRv1.ttf"}

func fontDirs() []string {
	var dirs []string

	if data := os.Getenv("XDG_DATA_HOME"); data != "" {
		dirs = append(dirs, filepath.Join(data, "fonts"))
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "fonts"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".fonts"))
	}

	shared := os.Getenv("XDG_DATA_DIRS")

	if shared == "" {
		shared = "/usr/local/share:/usr/share"
	}

	for _, d := range strings.Split(shared, ":") {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "fonts"))
		}
	}

	return dirs
}
