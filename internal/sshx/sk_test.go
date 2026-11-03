package sshx

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/pem"
	"testing"

	"golang.org/x/crypto/ssh"
)

func skString(b []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(b)))
	return append(out, b...)
}

func skPEM(cipher string) ([]byte, []byte) {
	pub, _, _ := ed25519.GenerateKey(nil)
	var pubBlob []byte
	pubBlob = append(pubBlob, skString([]byte(ssh.KeyAlgoSKED25519))...)
	pubBlob = append(pubBlob, skString(pub)...)
	pubBlob = append(pubBlob, skString([]byte("ssh:"))...)

	var priv []byte
	priv = binary.BigEndian.AppendUint32(priv, 42)
	priv = binary.BigEndian.AppendUint32(priv, 42)
	priv = append(priv, skString([]byte(ssh.KeyAlgoSKED25519))...)
	priv = append(priv, skString(pub)...)
	priv = append(priv, skString([]byte("ssh:"))...)
	priv = append(priv, 0x05)
	priv = append(priv, skString([]byte("handle"))...)
	priv = append(priv, skString(nil)...)
	priv = append(priv, skString([]byte("comment"))...)

	body := []byte(opensshMagic)
	body = append(body, skString([]byte(cipher))...)
	body = append(body, skString([]byte(cipher))...)
	body = append(body, skString(nil)...)
	body = binary.BigEndian.AppendUint32(body, 1)
	body = append(body, skString(pubBlob)...)
	body = append(body, skString(priv)...)

	return pem.EncodeToMemory(&pem.Block{Type: opensshPEM, Bytes: body}), pubBlob
}

func TestParseSK(t *testing.T) {
	data, pubBlob := skPEM(cipherNone)
	k, err := parseSK(data)

	if err != nil {
		t.Fatal(err)
	}

	if k.application != "ssh:" || k.flags != 0x05 || string(k.handle) != "handle" || string(k.pub.Marshal()) != string(pubBlob) {
		t.Fatalf("unexpected key %+v", k)
	}

	imported, err := ImportKey("yubikey", data, "")

	if err != nil || imported.Type != ssh.KeyAlgoSKED25519 {
		t.Fatalf("ImportKey: %v %+v", err, imported)
	}

	if _, err := parseSK([]byte("nope")); err != ErrNotSK {
		t.Fatalf("plain text: %v", err)
	}

	enc, _ := skPEM("aes256-ctr")

	if _, err := parseSK(enc); err != errSKEncrypted {
		t.Fatalf("encrypted: %v", err)
	}
}

func TestSKSignature(t *testing.T) {
	auth := make([]byte, authDataPrefix)
	auth[32] = 0x01
	binary.BigEndian.PutUint32(auth[33:], 7)

	sig, err := skSignature(ssh.KeyAlgoSKED25519, auth, []byte("raw"))

	if err != nil || sig.Format != ssh.KeyAlgoSKED25519 || string(sig.Blob) != "raw" || string(sig.Rest) != "\x01\x00\x00\x00\x07" {
		t.Fatalf("got %+v, %v", sig, err)
	}

	if _, err := skSignature(ssh.KeyAlgoSKED25519, auth[:10], nil); err == nil {
		t.Fatal("short authenticator data must fail")
	}
}

func authenticatorSign(t *testing.T, app string, data []byte, sign func([]byte) []byte) ([]byte, []byte) {
	t.Helper()
	rp := sha256.Sum256([]byte(app))
	auth := append(rp[:], 0x01, 0, 0, 0, 9)
	digest := sha256.Sum256(data)
	return auth, sign(append(append([]byte(nil), auth...), digest[:]...))
}

func TestSKSignatureVerifies(t *testing.T) {
	data := []byte("session data")
	edPub, edPriv, _ := ed25519.GenerateKey(rand.Reader)
	ecPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	point, _ := ecPriv.PublicKey.ECDH()

	cases := []struct {
		blob []byte
		sign func([]byte) []byte
	}{
		{
			blob: ssh.Marshal(struct{ Type, Key, App string }{ssh.KeyAlgoSKED25519, string(edPub), "ssh:"}),
			sign: func(m []byte) []byte { return ed25519.Sign(edPriv, m) },
		},
		{
			blob: ssh.Marshal(struct{ Type, Curve, Key, App string }{ssh.KeyAlgoSKECDSA256, "nistp256", string(point.Bytes()), "ssh:"}),
			sign: func(m []byte) []byte {
				h := sha256.Sum256(m)
				sig, _ := ecdsa.SignASN1(rand.Reader, ecPriv, h[:])
				return sig
			},
		},
	}

	for _, c := range cases {
		pub, err := ssh.ParsePublicKey(c.blob)

		if err != nil {
			t.Fatal(err)
		}

		auth, raw := authenticatorSign(t, "ssh:", data, c.sign)
		sig, err := skSignature(pub.Type(), auth, raw)

		if err != nil {
			t.Fatal(err)
		}

		if err := pub.Verify(data, sig); err != nil {
			t.Fatalf("%s: %v", pub.Type(), err)
		}
	}
}
