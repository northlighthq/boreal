//go:build !(linux && !android && !nowayland && cgo)

package dragdrop

import "unsafe"

// EnableWayland is a no-op where Boreal doesn't run on Wayland.
func EnableWayland(display, surface unsafe.Pointer, onDrop func(paths []string), onHover func(over bool)) {}
