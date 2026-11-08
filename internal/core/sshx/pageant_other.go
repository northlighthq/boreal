//go:build !windows

package sshx

import (
	"errors"
	"io"
)

func dialPageant() (io.ReadWriteCloser, error) {
	return nil, errors.New("pageant is only available on Windows")
}
