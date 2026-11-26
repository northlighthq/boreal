package webauthn


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
