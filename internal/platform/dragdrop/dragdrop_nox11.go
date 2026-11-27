//go:build !(linux && !android && !nox11 && cgo)

package dragdrop

func EnableX11(window uintptr, onDrop func(paths []string), onHover func(over bool)) {}
