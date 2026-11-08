package open

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

func File(path string) error {
	verb, err := windows.UTF16PtrFromString("open")

	if err != nil {
		return err
	}

	file, err := windows.UTF16PtrFromString(path)

	if err != nil {
		return err
	}

	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

func Edit(path string) error {
	return exec.Command("notepad.exe", path).Start()
}
