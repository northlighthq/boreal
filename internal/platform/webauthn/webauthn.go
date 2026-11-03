package webauthn

import "errors"

var ErrUnsupported = errors.New("security keys need Windows 10 1903 or later")

type Request struct {
	RPID       string
	ClientData []byte
	Credential []byte
	Verify     bool
}

type Assertion struct {
	AuthData  []byte
	Signature []byte
}
