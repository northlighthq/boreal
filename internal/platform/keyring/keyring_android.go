package keyring

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/platform/android"
)

const folder = "keyring"

func systemSet(id, label string, secret []byte) error {
	path, err := file(id)

	if err != nil {
		return err
	}

	sealed, err := android.Seal(secret)

	if err != nil {
		return ErrUnsupported
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	return config.WriteAtomic(path, sealed)
}

func systemGet(id string) ([]byte, error) {
	path, err := file(id)

	if err != nil {
		return nil, err
	}

	sealed, err := os.ReadFile(path)

	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return android.Open(sealed)
}

func systemDelete(id string) error {
	path, err := file(id)

	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}

func file(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return "", ErrNotFound
	}

	dir, err := config.Dir()

	if err != nil {
		return "", err
	}

	return filepath.Join(dir, folder, id), nil
}
