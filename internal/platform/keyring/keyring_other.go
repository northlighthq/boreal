//go:build !linux && !(darwin && cgo)

package keyring

func systemSet(id, label string, secret []byte) error {
	return ErrUnsupported
}

func systemGet(id string) ([]byte, error) {
	return nil, ErrUnsupported
}

func systemDelete(id string) error {
	return ErrUnsupported
}
