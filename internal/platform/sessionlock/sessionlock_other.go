//go:build !windows && !(linux && !android) && !(darwin && cgo)

package sessionlock

const Supported = false

func Locked() bool { return false }
