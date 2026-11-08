package sshx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

const (
	passwdFile  = "/etc/passwd"
	groupFile   = "/etc/group"
	maxNameFile = 4 << 20
	execBits    = 0o111
)

var errPathRequired = errors.New("sftp: empty path")

type treeErrors struct {
	first  error
	failed int
}

func (t *treeErrors) add(err error) {
	if err == nil {
		return
	}

	if t.first == nil {
		t.first = err
	}

	t.failed++
}

func (t *treeErrors) err() error {
	if t.failed <= 1 {
		return t.first
	}

	return fmt.Errorf("%d items couldn't be changed, the first one: %w", t.failed, t.first)
}

type Props struct {
	Path     string
	Mode     os.FileMode
	UID, GID uint32
	Size     int64
	ModTime  time.Time
	Accessed time.Time
	IsDir    bool
	IsLink   bool
	Target   string
}

type Names struct {
	Users  map[uint32]string
	Groups map[uint32]string
}

func (n Names) User(id uint32) string {
	if name, ok := n.Users[id]; ok {
		return name
	}

	return strconv.FormatUint(uint64(id), 10)
}

func (n Names) Group(id uint32) string {
	if name, ok := n.Groups[id]; ok {
		return name
	}

	return strconv.FormatUint(uint64(id), 10)
}

func (n Names) UserID(s string) (uint32, bool) {
	return lookup(n.Users, s)
}

func (n Names) GroupID(s string) (uint32, bool) {
	return lookup(n.Groups, s)
}

func lookup(m map[uint32]string, s string) (uint32, bool) {
	s = strings.TrimSpace(s)

	if id, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(id), true
	}

	for id, name := range m {
		if name == s {
			return id, true
		}
	}

	return 0, false
}

func (s *SFTP) Props(p string) (Props, error) {
	if p == "" {
		return Props{}, errPathRequired
	}

	fi, err := s.Lstat(p)

	if err != nil {
		return Props{}, err
	}

	pr := Props{Path: p, Mode: fi.Mode(), Size: fi.Size(), ModTime: fi.ModTime(), IsDir: fi.IsDir(), IsLink: fi.Mode()&os.ModeSymlink != 0}

	if st, ok := fi.Sys().(*sftp.FileStat); ok {
		pr.UID, pr.GID = st.UID, st.GID
		pr.Accessed = time.Unix(int64(st.Atime), 0)
	}

	if !pr.IsLink {
		return pr, nil
	}

	pr.Target, _ = s.ReadLink(p)

	if t, err := s.Stat(p); err == nil {
		pr.IsDir, pr.Mode = t.IsDir(), t.Mode()|os.ModeSymlink

		if st, ok := t.Sys().(*sftp.FileStat); ok {
			pr.UID, pr.GID = st.UID, st.GID
		}
	}

	return pr, nil
}

func (s *SFTP) Names() Names {
	return Names{Users: s.idFile(passwdFile, 2), Groups: s.idFile(groupFile, 2)}
}

func (s *SFTP) idFile(name string, field int) map[uint32]string {
	out := map[uint32]string{}
	f, err := s.Open(name)

	if err != nil {
		return out
	}

	defer f.Close()

	return parseIDs(bufio.NewScanner(io.LimitReader(f, maxNameFile)), field)
}

func parseIDs(sc *bufio.Scanner, field int) map[uint32]string {
	out := map[uint32]string{}

	for sc.Scan() {
		parts := strings.Split(sc.Text(), ":")

		if len(parts) <= field || strings.HasPrefix(parts[0], "#") {
			continue
		}

		id, err := strconv.ParseUint(parts[field], 10, 32)

		if err != nil {
			continue
		}

		if _, ok := out[uint32(id)]; !ok {
			out[uint32(id)] = parts[0]
		}
	}

	return out
}

type DirUsage struct {
	Size     int64
	Files    int
	Folders  int
	Complete bool
}

func (s *SFTP) DirSize(ctx context.Context, p string, limit int, progress func(DirUsage)) (DirUsage, error) {
	var u DirUsage
	w := s.Walk(p)

	for w.Step() {
		if ctx.Err() != nil {
			return u, ctx.Err()
		}

		if w.Err() != nil {
			continue
		}

		if w.Path() == p {
			continue
		}

		fi := w.Stat()

		switch {
		case fi.IsDir():
			u.Folders++
		case fi.Mode().IsRegular():
			u.Files++
			u.Size += fi.Size()
		}

		if u.Files+u.Folders >= limit {
			return u, nil
		}

		if progress != nil && (u.Files+u.Folders)%500 == 0 {
			progress(u)
		}
	}

	u.Complete = true
	return u, nil
}

func (s *SFTP) ChmodTree(p string, mode os.FileMode, recursive bool) error {
	if err := s.Chmod(p, mode); err != nil {
		return err
	}

	if !recursive {
		return nil
	}

	var errs treeErrors
	w := s.Walk(p)

	for w.Step() {
		if w.Path() == p {
			continue
		}

		if w.Err() != nil {
			errs.add(w.Err())
			continue
		}

		fi := w.Stat()

		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}

		errs.add(s.Chmod(w.Path(), TreeMode(mode, fi.IsDir(), fi.Mode())))
	}

	return errs.err()
}

func TreeMode(mode os.FileMode, dir bool, current os.FileMode) os.FileMode {
	if dir || current.Perm()&execBits != 0 {
		return mode
	}

	return mode &^ execBits
}

func (s *SFTP) ChownTree(p string, uid, gid uint32, recursive bool) error {
	if err := s.Chown(p, int(uid), int(gid)); err != nil {
		return err
	}

	if !recursive {
		return nil
	}

	var errs treeErrors
	w := s.Walk(p)

	for w.Step() {
		if w.Path() == p {
			continue
		}

		if w.Err() != nil {
			errs.add(w.Err())
			continue
		}

		if w.Stat().Mode()&os.ModeSymlink != 0 {
			continue
		}

		errs.add(s.Chown(w.Path(), int(uid), int(gid)))
	}

	return errs.err()
}

func FormatMode(m os.FileMode) string {
	n := uint64(m.Perm())

	if m&os.ModeSetuid != 0 {
		n |= 0o4000
	}

	if m&os.ModeSetgid != 0 {
		n |= 0o2000
	}

	if m&os.ModeSticky != 0 {
		n |= 0o1000
	}

	s := strconv.FormatUint(n, 8)

	for len(s) < 3 {
		s = "0" + s
	}

	return s
}

func ParseMode(s string) (os.FileMode, bool) {
	s = strings.TrimSpace(s)

	if len(s) < 3 || len(s) > 4 {
		return 0, false
	}

	n, err := strconv.ParseUint(s, 8, 32)

	if err != nil {
		return 0, false
	}

	m := os.FileMode(n & 0o777)

	if n&0o4000 != 0 {
		m |= os.ModeSetuid
	}

	if n&0o2000 != 0 {
		m |= os.ModeSetgid
	}

	if n&0o1000 != 0 {
		m |= os.ModeSticky
	}

	return m, true
}
