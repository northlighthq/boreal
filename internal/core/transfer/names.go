package transfer

import (
	"path/filepath"
	"strings"
)

var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func SafeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}

		return r
	}, name)
	name = strings.TrimRight(name, ". ")

	if name == "" {
		return "_"
	}

	if base, _, _ := strings.Cut(name, "."); reservedNames[strings.ToUpper(strings.TrimSpace(base))] {
		name = "_" + name
	}

	return name
}

func localPath(root, remoteRoot, remote string) (string, bool) {
	rest, ok := strings.CutPrefix(remote, remoteRoot)

	if !ok {
		return "", false
	}

	parts := []string{root}

	for _, p := range strings.Split(rest, "/") {
		if p != "" {
			parts = append(parts, SafeName(p))
		}
	}

	return filepath.Join(parts...), true
}
