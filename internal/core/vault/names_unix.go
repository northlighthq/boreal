//go:build !windows && !darwin

package vault

const (
	OSOwner = "system keyring"
	OSName  = "keyring"
)
