//go:build linux && !android

package urlhandler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const desktopFile = "boreal-links.desktop"

var schemes = []string{"ssh", "sftp"}

func executable() (string, error) {
	if p := os.Getenv("APPIMAGE"); p != "" {
		return p, nil
	}

	exe, err := os.Executable()

	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(exe)
}

func applicationsDir() (string, error) {
	data := os.Getenv("XDG_DATA_HOME")

	if data == "" {
		home, err := os.UserHomeDir()

		if err != nil {
			return "", err
		}

		data = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(data, "applications"), nil
}

func quoteExec(s string) string {
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", "$", `\\$`)
	return `"` + r.Replace(s) + `"`
}

func Registered() bool {
	dir, err := applicationsDir()

	if err != nil {
		return false
	}

	raw, err := os.ReadFile(filepath.Join(dir, desktopFile))
	exe, exeErr := executable()

	if err != nil || exeErr != nil || !strings.Contains(string(raw), quoteExec(exe)) {
		return false
	}

	for _, s := range schemes {
		out, err := exec.Command("xdg-mime", "query", "default", "x-scheme-handler/"+s).Output()

		if err != nil || strings.TrimSpace(string(out)) != desktopFile {
			return false
		}
	}

	return true
}

func Set(enabled bool) error {
	dir, err := applicationsDir()

	if err != nil {
		return err
	}

	path := filepath.Join(dir, desktopFile)

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}

		refresh(dir)
		return nil
	}

	exe, err := executable()

	if err != nil {
		return err
	}

	mimes := make([]string, len(schemes))

	for i, s := range schemes {
		mimes[i] = "x-scheme-handler/" + s
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
		return err
	}

	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return err
	}

	refresh(dir)

	return exec.Command("xdg-mime", append([]string{"default", desktopFile}, mimes...)...).Run()
}

func refresh(dir string) {
	if _, err := exec.LookPath("update-desktop-database"); err == nil {
		exec.Command("update-desktop-database", dir).Run()
	}
}
