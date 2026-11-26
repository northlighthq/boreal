//go:build !windows && !(darwin && cgo) && !(linux && !android)

package hello

const Name = "System sign-in"

func Available() bool { return false }

func Verify(hwnd uintptr, message string) error { return ErrUnavailable }
