//go:build !windows

package sessionlock

func Locked() bool { return false }
