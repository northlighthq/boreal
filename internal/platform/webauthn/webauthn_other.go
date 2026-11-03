//go:build !windows

package webauthn

func Assert(r Request) (Assertion, error) {
	return Assertion{}, ErrUnsupported
}
