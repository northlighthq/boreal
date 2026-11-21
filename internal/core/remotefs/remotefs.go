package remotefs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

const (
	MaxRead    = 1 << 20
	MaxWrite   = 1 << 20
	MaxEntries = 500
	newMode    = 0o644
	tmpPrefix  = ".boreal-tmp-"
	sshDir     = ".ssh"
	keysPrefix = "authorized_keys"
	maxHops    = 40
)

const (
	TypeFile  = "file"
	TypeDir   = "folder"
	TypeLink  = "link"
	TypeOther = "other"
)

var (
	ErrEmptyPath = errors.New("give the path of a file or folder")
	ErrIsDir     = errors.New("this path is a folder, not a file")
	ErrNotDir    = errors.New("this path isn't a folder")
	ErrNoParent  = errors.New("the folder this file would go in doesn't exist")
	ErrExists    = errors.New("the file already exists. Pass overwrite to replace it")
	ErrLink      = errors.New("this path is a link, so Boreal won't overwrite it. Write to the file it points to instead")
	ErrSpecial   = errors.New("this path isn't a regular file, so Boreal won't overwrite it")
	ErrProtected = errors.New("Boreal won't write SSH sign-in files (anything in ~/.ssh or named authorized_keys). Use grant_access and revoke_access to change who can sign in")
	ErrTooBig    = fmt.Errorf("the content is larger than %d KB", MaxWrite>>10)
	errLoop      = errors.New("the path goes through too many links")
)

type Entry struct {
	Name     string
	Type     string
	Size     int64
	Mode     fs.FileMode
	Modified time.Time
}

type File struct {
	Path      string
	Size      int64
	Data      []byte
	Truncated bool
}

type Written struct {
	Path    string
	Bytes   int
	Created bool
}

type FS interface {
	List(dir string, limit int) (string, []Entry, bool, error)
	Read(p string, offset int64, limit int) (File, error)
	Write(p string, data []byte, overwrite bool) (Written, error)
}

type remote struct {
	c *sftp.Client
}

func New(c *sftp.Client) FS {
	return remote{c: c}
}

func kind(m fs.FileMode) string {
	switch {
	case m.IsDir():
		return TypeDir
	case m&fs.ModeSymlink != 0:
		return TypeLink
	case m.IsRegular():
		return TypeFile
	}

	return TypeOther
}

func clean(p string) (string, error) {
	p = strings.TrimSpace(p)

	if p == "" {
		return "", ErrEmptyPath
	}

	return path.Clean(p), nil
}

func (r remote) List(dir string, limit int) (string, []Entry, bool, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}

	dir, err := clean(dir)

	if err != nil {
		return "", nil, false, err
	}

	real, err := r.c.RealPath(dir)

	if err != nil {
		return "", nil, false, err
	}

	infos, err := r.c.ReadDir(real)

	if err != nil {
		if info, serr := r.c.Stat(real); serr == nil && !info.IsDir() {
			return "", nil, false, ErrNotDir
		}

		return "", nil, false, err
	}

	if limit <= 0 || limit > MaxEntries {
		limit = MaxEntries
	}

	out := make([]Entry, 0, min(len(infos), limit))

	for _, fi := range infos {
		out = append(out, Entry{Name: fi.Name(), Type: kind(fi.Mode()), Size: fi.Size(), Mode: fi.Mode().Perm(), Modified: fi.ModTime()})
	}

	slices.SortFunc(out, func(a, b Entry) int {
		if (a.Type == TypeDir) != (b.Type == TypeDir) {
			if a.Type == TypeDir {
				return -1
			}

			return 1
		}

		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	if len(out) > limit {
		return real, out[:limit], true, nil
	}

	return real, out, false, nil
}

func (r remote) Read(p string, offset int64, limit int) (File, error) {
	p, err := clean(p)

	if err != nil {
		return File{}, err
	}

	if limit <= 0 || limit > MaxRead {
		limit = MaxRead
	}

	offset = max(offset, 0)
	real, err := r.c.RealPath(p)

	if err != nil {
		return File{}, err
	}

	info, err := r.c.Stat(real)

	if err != nil {
		return File{}, err
	}

	if info.IsDir() {
		return File{}, ErrIsDir
	}

	f, err := r.c.Open(real)

	if err != nil {
		return File{}, err
	}

	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return File{}, err
	}

	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))

	if err != nil {
		return File{}, err
	}

	out := File{Path: real, Size: info.Size(), Data: data}

	if len(data) > limit {
		out.Data, out.Truncated = data[:limit], true
	}

	return out, nil
}

func protected(p string) bool {
	for part := range strings.SplitSeq(strings.ToLower(p), "/") {
		if part == sshDir || strings.HasPrefix(part, keysPrefix) {
			return true
		}
	}

	return false
}

func (r remote) Write(p string, data []byte, overwrite bool) (Written, error) {
	p, err := clean(p)

	if err != nil {
		return Written{}, err
	}

	if len(data) > MaxWrite {
		return Written{}, ErrTooBig
	}

	if protected(p) {
		return Written{}, ErrProtected
	}

	base := path.Base(p)

	if base == "." || base == "/" || base == ".." {
		return Written{}, ErrIsDir
	}

	dir, err := r.resolve(path.Dir(p))

	if errors.Is(err, errLoop) {
		return Written{}, err
	}

	if err != nil {
		return Written{}, ErrNoParent
	}

	if info, err := r.c.Stat(dir); err != nil || !info.IsDir() {
		return Written{}, ErrNoParent
	}

	target := path.Join(dir, base)

	if protected(target) {
		return Written{}, ErrProtected
	}

	mode, created := fs.FileMode(newMode), false
	info, err := r.c.Lstat(target)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		created = true
	case err != nil:
		return Written{}, err
	case info.Mode()&fs.ModeSymlink != 0:
		return Written{}, ErrLink
	case info.IsDir():
		return Written{}, ErrIsDir
	case !info.Mode().IsRegular():
		return Written{}, ErrSpecial
	case !overwrite:
		return Written{}, ErrExists
	default:
		mode = info.Mode().Perm()
	}

	tmp, err := r.temp(dir, data, mode)

	if err != nil {
		return Written{}, err
	}

	if err := r.c.PosixRename(tmp, target); err != nil {
		r.c.Remove(tmp)
		return Written{}, err
	}

	return Written{Path: target, Bytes: len(data), Created: created}, nil
}

func split(p string) (string, []string) {
	root := "/"

	if !strings.HasPrefix(p, "/") {
		root, p, _ = strings.Cut(p, "/")
	}

	var parts []string

	for part := range strings.SplitSeq(p, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}

	return root, parts
}

func drivePath(t string) bool {
	if len(t) < 3 || t[1] != ':' || (t[2] != '\\' && t[2] != '/') {
		return false
	}

	c := t[0] | 0x20
	return c >= 'a' && c <= 'z'
}

func windowsTarget(t string) string {
	d := t

	for _, prefix := range []string{`\\?\`, `\??\`} {
		d = strings.TrimPrefix(d, prefix)
	}

	if !drivePath(d) {
		return t
	}

	return "/" + strings.ReplaceAll(d, `\`, "/")
}

func (r remote) resolve(p string) (string, error) {
	abs, err := r.c.RealPath(p)

	if err != nil {
		return "", err
	}

	root, parts := split(path.Clean(abs))
	cur, hops := root, 0

	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]

		if part == ".." {
			cur = path.Dir(cur)
			continue
		}

		next := path.Join(cur, part)
		info, err := r.c.Lstat(next)

		if err != nil {
			return "", err
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			cur = next
			continue
		}

		if hops++; hops > maxHops {
			return "", errLoop
		}

		target, err := r.c.ReadLink(next)

		if err != nil {
			return "", err
		}

		target = windowsTarget(target)

		if !path.IsAbs(target) {
			target = path.Join(cur, target)
		}

		var rest []string
		cur, rest = split(path.Clean(target))
		parts = append(rest, parts...)
	}

	return cur, nil
}

func (r remote) temp(dir string, data []byte, mode fs.FileMode) (string, error) {
	var b [8]byte
	rand.Read(b[:])
	tmp := path.Join(dir, tmpPrefix+hex.EncodeToString(b[:]))
	f, err := r.c.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)

	if err != nil {
		return "", err
	}

	cherr := r.c.Chmod(tmp, mode)
	var werr error

	if cherr == nil {
		_, werr = f.Write(data)
	}

	if err := errors.Join(cherr, werr, f.Close()); err != nil {
		r.c.Remove(tmp)
		return "", err
	}

	return tmp, nil
}
