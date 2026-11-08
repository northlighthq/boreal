//go:build !windows

package sshx

import (
	"os/exec"
	"strings"
)

func quoteArg(v string) (string, bool) {
	return "'" + strings.ReplaceAll(v, "'", `'''`) + "'", true
}

func shellCommand(line string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", "exec "+line)
}
