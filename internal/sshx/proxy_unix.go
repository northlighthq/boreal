//go:build !windows

package sshx

import "os/exec"

func shellCommand(line string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", "exec "+line)
}
