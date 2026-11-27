//go:build !windows && !darwin

package open

import (
	"errors"
	"os/exec"
	"strings"
)

const (
	xdgOpen   = "xdg-open"
	xdgMime   = "xdg-mime"
	gtkLaunch = "gtk-launch"
	plainText = "text/plain"
)

var errNoEditor = errors.New("no default text editor is set")

func File(path string) error {
	return start(exec.Command(xdgOpen, path))
}

func URL(link string) error {
	return start(exec.Command(xdgOpen, link))
}

func Edit(path string) error {
	out, err := exec.Command(xdgMime, "query", "default", plainText).Output()
	app := strings.TrimSpace(string(out))

	if err != nil || app == "" {
		return errNoEditor
	}

	return start(exec.Command(gtkLaunch, app, path))
}
