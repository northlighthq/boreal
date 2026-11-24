//go:build !windows && (!linux || android)

package sessionlock

const Supported = false

func Locked() bool { return false }
