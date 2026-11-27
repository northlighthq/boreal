package open

import "os/exec"

const (
	opener   = "open"
	textFlag = "-t"
)

func File(path string) error {
	return start(exec.Command(opener, path))
}

func URL(link string) error {
	return start(exec.Command(opener, link))
}

func Edit(path string) error {
	return start(exec.Command(opener, textFlag, path))
}
