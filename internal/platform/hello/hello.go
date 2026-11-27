package hello

import "errors"

var (
	ErrUnavailable = errors.New(Name + " isn't set up on this device")
	ErrCanceled    = errors.New(Name + " was cancelled")
	ErrFailed      = errors.New(Name + " couldn't verify you")
)
