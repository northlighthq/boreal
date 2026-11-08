package sshx

import (
	"bytes"
	"encoding/asn1"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"math/big"

	"golang.org/x/crypto/ssh"

	"github.com/northlighthq/boreal/internal/platform/webauthn"
)

const (
	opensshMagic   = "openssh-key-v1\x00"
	opensshPEM     = "OPENSSH PRIVATE KEY"
	cipherNone     = "none"
	skUserVerify   = 0x04
	authDataPrefix = 37
)

var (
	ErrNotSK          = errors.New("not a hardware key")
	errSKEncrypted    = errors.New("hardware key files with a passphrase aren't supported: remove it with ssh-keygen -p, the token already protects the key")
	errSKMalformed    = errors.New("the hardware key file is damaged")
	errSKShortReply   = errors.New("the security key returned an incomplete signature")
	errSKUnknownCurve = errors.New("unsupported security key type")
)

type skKey struct {
	pub         ssh.PublicKey
	application string
	flags       byte
	handle      []byte
}

type skSigner struct {
	key    skKey
	notify func(string)
}

func IsSK(keyType string) bool {
	return keyType == ssh.KeyAlgoSKED25519 || keyType == ssh.KeyAlgoSKECDSA256
}

func parseSK(pemData []byte) (skKey, error) {
	block, _ := pem.Decode(pemData)

	if block == nil || block.Type != opensshPEM || !bytes.HasPrefix(block.Bytes, []byte(opensshMagic)) {
		return skKey{}, ErrNotSK
	}

	r := &reader{b: block.Bytes[len(opensshMagic):]}
	cipher, kdf := r.bytes(), r.bytes()
	r.bytes()
	count := r.u32()
	pubBlob := r.bytes()
	priv := r.bytes()

	if r.err != nil || count != 1 {
		return skKey{}, errSKMalformed
	}

	pub, err := ssh.ParsePublicKey(pubBlob)

	if err != nil || !IsSK(pub.Type()) {
		return skKey{}, ErrNotSK
	}

	if string(cipher) != cipherNone || string(kdf) != cipherNone {
		return skKey{}, errSKEncrypted
	}

	p := &reader{b: priv}

	if p.u32() != p.u32() {
		return skKey{}, errSKMalformed
	}

	typ := string(p.bytes())

	switch typ {
	case ssh.KeyAlgoSKED25519:
		p.bytes()
	case ssh.KeyAlgoSKECDSA256:
		p.bytes()
		p.bytes()
	default:
		return skKey{}, errSKUnknownCurve
	}

	k := skKey{pub: pub, application: string(p.bytes()), flags: p.byte(), handle: p.bytes()}

	if p.err != nil || k.application == "" || len(k.handle) == 0 {
		return skKey{}, errSKMalformed
	}

	return k, nil
}

func (s *skSigner) PublicKey() ssh.PublicKey {
	return s.key.pub
}

func (s *skSigner) Sign(_ io.Reader, data []byte) (*ssh.Signature, error) {
	if s.notify != nil {
		s.notify("Touch your security key to sign in…")
	}

	a, err := webauthn.Assert(webauthn.Request{
		RPID:       s.key.application,
		ClientData: data,
		Credential: s.key.handle,
		Verify:     s.key.flags&skUserVerify != 0,
	})

	if err != nil {
		return nil, err
	}

	return skSignature(s.key.pub.Type(), a.AuthData, a.Signature)
}

func skSignature(keyType string, authData, sig []byte) (*ssh.Signature, error) {
	if len(authData) < authDataPrefix {
		return nil, errSKShortReply
	}

	blob := sig

	if keyType == ssh.KeyAlgoSKECDSA256 {
		var rs struct{ R, S *big.Int }

		if _, err := asn1.Unmarshal(sig, &rs); err != nil {
			return nil, errSKShortReply
		}

		blob = ssh.Marshal(rs)
	}

	rest := make([]byte, 5)
	rest[0] = authData[32]
	copy(rest[1:], authData[33:37])
	return &ssh.Signature{Format: keyType, Blob: blob, Rest: rest}, nil
}

type reader struct {
	b   []byte
	err error
}

func (r *reader) u32() uint32 {
	if r.err != nil || len(r.b) < 4 {
		r.err = errSKMalformed
		return 0
	}

	v := binary.BigEndian.Uint32(r.b)
	r.b = r.b[4:]
	return v
}

func (r *reader) byte() byte {
	if r.err != nil || len(r.b) < 1 {
		r.err = errSKMalformed
		return 0
	}

	v := r.b[0]
	r.b = r.b[1:]
	return v
}

func (r *reader) bytes() []byte {
	n := r.u32()

	if r.err != nil || uint32(len(r.b)) < n {
		r.err = errSKMalformed
		return nil
	}

	v := r.b[:n]
	r.b = r.b[n:]
	return v
}
