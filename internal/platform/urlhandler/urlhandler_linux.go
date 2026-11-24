//go:build linux && !android

package urlhandler

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	Supported   = true
	Hint        = "If another app keeps opening them, choose Boreal for ssh links in your desktop's default applications."
	desktopFile = "boreal-url-handler.desktop"
)

var schemes = []string{"ssh", "sftp"}

func mimeType(scheme string) string {
	return "x-scheme-handler/" + scheme
}

func entryPath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")

	if dir == "" {
		home, err := os.UserHomeDir()

		if err != nil {
			return "", err
		}

		dir = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dir, "applications", desktopFile), nil
}

func quoteExec(path string) string {
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", "$", `\\$`, "%", "%%")
	return `"` + r.Replace(path) + `"`
}

func Registered() bool {
	path, err := entryPath()

	if err != nil {
		return false
	}

	if _, err := os.Stat(path); err != nil {
		return false
	}

	for _, s := range schemes {
		out, err := exec.Command("xdg-mime", "query", "default", mimeType(s)).Output()

		if err != nil || strings.TrimSpace(string(out)) != desktopFile {
			return false
		}
	}

	return true
}

func Set(enabled bool) error {
	path, err := entryPath()

	if err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}

		refresh(filepath.Dir(path))
		return nil
	}

	if _, err := exec.LookPath("xdg-mime"); err != nil {
		return errors.New("xdg-mime isn't installed, so Boreal can't register for ssh:// links")
	}

	exe, err := os.Executable()

	if err != nil {
		return err
	}

	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}

	mimes := make([]string, len(schemes))

	for i, s := range schemes {
		mimes[i] = mimeType(s)
	}

	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Boreal\nExec=%s %%u\nIcon=boreal\nTerminal=false\nNoDisplay=true\nMimeType=%s;\n", quoteExec(exe), strings.Join(mimes, ";"))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return err
	}

	refresh(filepath.Dir(path))

	for _, m := range mimes {
		if out, err := exec.Command("xdg-mime", "default", desktopFile, m).CombinedOutput(); err != nil {
			return fmt.Errorf("xdg-mime: %s", strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func refresh(dir string) {
	if _, err := exec.LookPath("update-desktop-database"); err == nil {
		exec.Command("update-desktop-database", dir).Run()
	}
}
