//go:build !(linux && !android && !nox11 && cgo)

package dragdrop

// EnableX11 is a no-op where Boreal doesn't run on X11.
func EnableX11(window uintptr, onDrop func(paths []string), onHover func(over bool)) {}
