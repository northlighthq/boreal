//go:build !windows && !darwin

package open

import (
	"errors"
	"os/exec"
	"strings"
)

var errNoEditor = errors.New("no default text editor is set")

func File(path string) error {
	return exec.Command("xdg-open", path).Start()
}

func Edit(path string) error {
	out, err := exec.Command("xdg-mime", "query", "default", "text/plain").Output()
	app := strings.TrimSpace(string(out))

	if err != nil || app == "" {
		return errNoEditor
	}

	return exec.Command("gtk-launch", app, path).Start()
}
