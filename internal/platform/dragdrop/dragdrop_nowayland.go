//go:build !(linux && !android && !nowayland && cgo)

package dragdrop

import "unsafe"

func EnableWayland(display, surface unsafe.Pointer, onDrop func(paths []string), onHover func(over bool)) {
}
