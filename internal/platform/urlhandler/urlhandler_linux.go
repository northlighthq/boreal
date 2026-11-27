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
	Supported    = true
	Hint         = "If another app keeps opening them, choose Boreal for ssh links in your desktop's default apps."
	desktopFile  = "boreal-links.desktop"
	schemePrefix = "x-scheme-handler/"
	xdgMime      = "xdg-mime"
	updateDB     = "update-desktop-database"
	appImageEnv  = "APPIMAGE"
	dataHomeEnv  = "XDG_DATA_HOME"
)

var errNoXdgMime = errors.New("Boreal needs xdg-mime (from xdg-utils) to open links")

var schemes = []string{"ssh", "sftp"}

func executable() (string, error) {
	if path := os.Getenv(appImageEnv); path != "" {
		return path, nil
	}

	exe, err := os.Executable()

	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(exe)
}

func applicationsDir() (string, error) {
	data := os.Getenv(dataHomeEnv)

	if data == "" {
		home, err := os.UserHomeDir()

		if err != nil {
			return "", err
		}

		data = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(data, "applications"), nil
}

func quoteExec(raw string) string {
	replacer := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", "$", `\\$`)
	return `"` + replacer.Replace(raw) + `"`
}

func Registered() bool {
	dir, err := applicationsDir()

	if err != nil {
		return false
	}

	entry, err := os.ReadFile(filepath.Join(dir, desktopFile))
	exe, exeErr := executable()

	if err != nil || exeErr != nil || !strings.Contains(string(entry), quoteExec(exe)) {
		return false
	}

	for _, scheme := range schemes {
		handler, err := exec.Command(xdgMime, "query", "default", schemePrefix+scheme).Output()

		if err != nil || strings.TrimSpace(string(handler)) != desktopFile {
			return false
		}
	}

	return true
}

func Set(enabled bool) error {
	dir, err := applicationsDir()

	if err != nil {
		return fmt.Errorf("urlhandler: data dir: %w", err)
	}

	path := filepath.Join(dir, desktopFile)

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("urlhandler: remove: %w", err)
		}

		refresh(dir)
		return nil
	}

	if _, err := exec.LookPath(xdgMime); err != nil {
		return errNoXdgMime
	}

	exe, err := executable()

	if err != nil {
		return fmt.Errorf("urlhandler: executable: %w", err)
	}

	mimes := make([]string, len(schemes))

	for index, scheme := range schemes {
		mimes[index] = schemePrefix + scheme
	}

	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Boreal\n" +
		"Comment=Open SSH and SFTP links in Boreal\n" +
		"Exec=" + quoteExec(exe) + " %u\n" +
		"Icon=boreal\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"MimeType=" + strings.Join(mimes, ";") + ";\n"

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("urlhandler: create dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return fmt.Errorf("urlhandler: write entry: %w", err)
	}

	refresh(dir)

	if err := exec.Command(xdgMime, append([]string{"default", desktopFile}, mimes...)...).Run(); err != nil {
		return fmt.Errorf("urlhandler: set default: %w", err)
	}

	return nil
}

func refresh(dir string) {
	if _, err := exec.LookPath(updateDB); err == nil {
		exec.Command(updateDB, dir).Run()
	}
}
