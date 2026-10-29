package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/northlighthq/boreal/internal/vault"
)

func GenerateEd25519(label, passphrase string) (vault.Key, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		return vault.Key{}, err
	}

	var block *pem.Block

	if passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, label, []byte(passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, label)
	}

	if err != nil {
		return vault.Key{}, err
	}

	sshPub, err := ssh.NewPublicKey(pub)

	if err != nil {
		return vault.Key{}, err
	}

	return vault.Key{
		Label:      label,
		Type:       sshPub.Type(),
		Private:    string(pem.EncodeToMemory(block)),
		Public:     strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " " + label,
		Passphrase: passphrase,
	}, nil
}

func ImportKey(label string, pemData []byte, passphrase string) (vault.Key, error) {
	var signer ssh.Signer
	var err error

	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pemData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(pemData)
	}

	if err != nil {
		return vault.Key{}, err
	}

	pub := signer.PublicKey()
	return vault.Key{
		Label:      label,
		Type:       pub.Type(),
		Private:    string(pemData),
		Public:     strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))) + " " + label,
		Passphrase: passphrase,
	}, nil
}

func IsPassphraseMissing(err error) bool {
	var pm *ssh.PassphraseMissingError
	return errors.As(err, &pm)
}

func ParseKey(k vault.Key, hooks Hooks) (ssh.Signer, error) {
	if k.Passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(k.Private), []byte(k.Passphrase))
	}

	signer, err := ssh.ParsePrivateKey([]byte(k.Private))

	if err == nil || !IsPassphraseMissing(err) {
		return signer, err
	}

	if hooks.KeyPassphrase != nil {
		pass, ok := hooks.KeyPassphrase(k)

		if !ok {
			return nil, errors.New("cancelled")
		}

		return ssh.ParsePrivateKeyWithPassphrase([]byte(k.Private), []byte(pass))
	}

	if hooks.AskPassword == nil {
		return nil, err
	}

	pass, ok := hooks.AskPassword(fmt.Sprintf("Passphrase for key %q", k.Label))

	if !ok {
		return nil, errors.New("cancelled")
	}

	return ssh.ParsePrivateKeyWithPassphrase([]byte(k.Private), []byte(pass))
}

func Fingerprint(public string) string {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(public))

	if err != nil {
		return ""
	}

	return ssh.FingerprintSHA256(pub)
}

func CheckPassphrase(k vault.Key, passphrase string) error {
	_, err := ssh.ParsePrivateKeyWithPassphrase([]byte(k.Private), []byte(passphrase))
	return err
}

var ErrCertMismatch = errors.New("the certificate doesn't belong to this key")

func ParseCertificate(text string) (*ssh.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(text))

	if err != nil {
		return nil, err
	}

	cert, ok := pub.(*ssh.Certificate)

	if !ok {
		return nil, errors.New("this file isn't an SSH certificate")
	}

	return cert, nil
}

func CheckCertificate(k vault.Key, text string) error {
	cert, err := ParseCertificate(text)

	if err != nil {
		return err
	}

	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.Public))

	if err != nil {
		return err
	}

	if string(cert.Key.Marshal()) != string(pub.Marshal()) {
		return ErrCertMismatch
	}

	return nil
}

func Signers(k vault.Key, hooks Hooks) ([]ssh.Signer, error) {
	signer, err := ParseKey(k, hooks)

	if err != nil {
		return nil, err
	}

	if k.Certificate == "" {
		return []ssh.Signer{signer}, nil
	}

	cert, err := ParseCertificate(k.Certificate)

	if err != nil {
		return []ssh.Signer{signer}, nil
	}

	certSigner, err := ssh.NewCertSigner(cert, signer)

	if err != nil {
		return []ssh.Signer{signer}, nil
	}

	return []ssh.Signer{certSigner, signer}, nil
}
