package sshx

import (
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

type Entry struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}

type SFTP struct {
	*sftp.Client
}

func OpenSFTP(c *Client) (*SFTP, error) {
	sc, err := sftp.NewClient(c.Client)

	if err != nil {
		return nil, err
	}

	return &SFTP{Client: sc}, nil
}

func (s *SFTP) List(dir string) ([]Entry, error) {
	infos, err := s.ReadDir(dir)

	if err != nil {
		return nil, err
	}

	out := make([]Entry, 0, len(infos))

	for _, fi := range infos {
		out = append(out, Entry{
			Name:    fi.Name(),
			Size:    fi.Size(),
			Mode:    fi.Mode(),
			ModTime: fi.ModTime(),
			IsDir:   fi.IsDir(),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}

		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *SFTP) Upload(r io.Reader, dst string) (int64, error) {
	f, err := s.Create(dst)

	if err != nil {
		return 0, err
	}

	defer f.Close()

	return io.Copy(f, r)
}

func (s *SFTP) Download(src string, w io.Writer) (int64, error) {
	f, err := s.Open(src)

	if err != nil {
		return 0, err
	}

	defer f.Close()

	return f.WriteTo(w)
}

func (s *SFTP) RemoveAll(p string) error {
	fi, err := s.Stat(p)

	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return s.Remove(p)
	}

	entries, err := s.ReadDir(p)

	if err != nil {
		return err
	}

	for _, e := range entries {
		if err := s.RemoveAll(path.Join(p, e.Name())); err != nil {
			return err
		}
	}

	return s.RemoveDirectory(p)
}
