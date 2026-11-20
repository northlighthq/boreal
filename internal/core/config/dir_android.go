package config

import (
	"os"
	"path/filepath"
)

const filesDir = "files"

func privateDir() (string, bool) {
	home := os.Getenv("HOME")

	if home == "" || filepath.Base(home) != filesDir {
		return "", false
	}

	return filepath.Join(filepath.Dir(home), "no_backup", appDir), true
}
