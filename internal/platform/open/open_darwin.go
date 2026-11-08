package open

import (
	"os/exec"
)

func File(path string) error {
	return exec.Command("open", path).Start()
}

func Edit(path string) error {
	return exec.Command("open", "-t", path).Start()
}
