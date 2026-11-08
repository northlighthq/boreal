package sshx

import (
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func quoteArg(v string) (string, bool) {
	for _, r := range v {
		safe := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-:@[] ", r)

		if !safe {
			return "", false
		}
	}

	if strings.Contains(v, " ") {
		return `"` + v + `"`, true
	}

	return v, true
}

func shellCommand(line string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd.exe /d /s /c "` + line + `"`,
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	return cmd
}
