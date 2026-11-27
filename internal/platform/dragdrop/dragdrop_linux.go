//go:build linux && !android && cgo

package dragdrop

import "errors"

// Enable is a no-op on Linux: drops arrive through EnableWayland or EnableX11.
func Enable(hwnd uintptr, onDrop func(paths []string), onHover func(over bool)) {}

func DragOut(paths []string) error {
	return errors.ErrUnsupported
}

func DragVirtual(files []VirtualFile) error {
	return errors.ErrUnsupported
}
