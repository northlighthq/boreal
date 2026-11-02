package sshx

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func shellCommand(line string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd.exe /d /s /c "` + line + `"`,
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	return cmd
}
