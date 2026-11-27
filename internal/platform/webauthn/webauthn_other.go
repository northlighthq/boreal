//go:build !windows

package webauthn

import "errors"

var ErrUnsupported = errors.New("load this security key into your SSH agent (ssh-add) and sign in with the agent")

func Assert(r Request) (Assertion, error) {
	return Assertion{}, ErrUnsupported
}
