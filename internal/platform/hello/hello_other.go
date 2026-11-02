//go:build !windows

package hello

func Available() bool { return false }

func Verify(hwnd uintptr, message string) error { return ErrUnavailable }
