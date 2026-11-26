//go:build !windows

package webauthn

import "errors"

// ErrUnsupported explains how to use a security key where Boreal can't talk to it directly.
var ErrUnsupported = errors.New("load this security key into your SSH agent (ssh-add) and sign in with the agent")

func Assert(r Request) (Assertion, error) {
	return Assertion{}, ErrUnsupported
}
