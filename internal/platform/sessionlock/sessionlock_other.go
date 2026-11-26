//go:build !windows && !(linux && !android) && !(darwin && cgo)

package sessionlock

func Locked() bool { return false }
