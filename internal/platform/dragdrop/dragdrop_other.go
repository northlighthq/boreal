//go:build !windows

package dragdrop

import "errors"

func Enable(hwnd uintptr, onDrop func(paths []string), onHover func(over bool)) {}

func DragOut(paths []string) error {
	return errors.ErrUnsupported
}

func DragVirtual(files []VirtualFile) error {
	return errors.ErrUnsupported
}
