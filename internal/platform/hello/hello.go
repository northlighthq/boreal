// Package hello asks Windows Hello (PIN, face or fingerprint) to confirm
// that the user is present.
package hello

import "errors"

var (
	ErrUnavailable = errors.New("Windows Hello isn't set up on this PC")
	ErrCanceled    = errors.New("Windows Hello was cancelled")
	ErrFailed      = errors.New("Windows Hello couldn't verify you")
)
