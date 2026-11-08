package config

import (
	"os"
	"path/filepath"
)

func privateDir() (string, bool) {
	home := os.Getenv("HOME")

	if home == "" {
		return "", false
	}

	return filepath.Join(filepath.Dir(home), "no_backup", appDir), true
}
