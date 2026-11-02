//go:build !windows

// Package sessionlock tells whether the user's desktop session is locked.
package sessionlock

// Locked reports whether the desktop session is locked. Other systems don't
// report it yet.
func Locked() bool { return false }
