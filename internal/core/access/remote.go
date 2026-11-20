package access

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/pkg/sftp"
)

const (
	sshDir         = ".ssh"
	authorizedName = "authorized_keys"
	backupSuffix   = ".boreal-backup"
	tmpSuffix      = ".boreal-tmp"
	maxFileSize    = 1 << 20
	dirMode        = 0o700
	fileMode       = 0o600
)

var (
	ErrChanged = errors.New("authorized_keys changed on the server while Boreal was editing it; try again")
	ErrLink    = errors.New("authorized_keys is a link on the server, so Boreal won't rewrite it")
	ErrTooBig  = errors.New("authorized_keys is larger than 1 MB, so Boreal won't edit it")
)

type Files interface {
	Read() ([]byte, error)
	Replace(old, next []byte) error
}

type remote struct {
	c *sftp.Client
}

func Remote(c *sftp.Client) Files {
	return remote{c: c}
}

func authorizedPath() string {
	return path.Join(sshDir, authorizedName)
}

func (r remote) Read() ([]byte, error) {
	p := authorizedPath()
	info, err := r.c.Lstat(p)

	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, ErrLink
	}

	if info.Size() > maxFileSize {
		return nil, ErrTooBig
	}

	f, err := r.c.Open(p)

	if err != nil {
		return nil, err
	}

	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))

	if err != nil {
		return nil, err
	}

	if len(data) > maxFileSize {
		return nil, ErrTooBig
	}

	return data, nil
}

func (r remote) Replace(old, next []byte) error {
	current, err := r.Read()

	if err != nil {
		return err
	}

	if !bytes.Equal(current, old) {
		return ErrChanged
	}

	if err := r.ensureDir(); err != nil {
		return err
	}

	p := authorizedPath()

	if current != nil {
		if err := r.write(p+backupSuffix, current); err != nil {
			return fmt.Errorf("backing up authorized_keys: %w", err)
		}
	}

	tmp := p + tmpSuffix

	if err := r.write(tmp, next); err != nil {
		r.c.Remove(tmp)
		return err
	}

	if err := r.c.PosixRename(tmp, p); err != nil {
		r.c.Remove(tmp)
		return fmt.Errorf("replacing authorized_keys: %w", err)
	}

	return nil
}

func (r remote) ensureDir() error {
	info, err := r.c.Lstat(sshDir)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := r.c.Mkdir(sshDir); err != nil {
			return err
		}

		return r.c.Chmod(sshDir, dirMode)
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("%s on the server isn't a folder", sshDir)
	}

	return nil
}

func (r remote) write(p string, data []byte) error {
	f, err := r.c.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)

	if err != nil {
		return err
	}

	if err := f.Chmod(fileMode); err != nil {
		f.Close()
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}
