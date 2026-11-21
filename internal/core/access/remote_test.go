package access

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

func sftpPair(t *testing.T) (*sftp.Client, string) {
	t.Helper()
	home := t.TempDir()
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()

	srv, err := sftp.NewServer(struct {
		io.Reader
		io.WriteCloser
	}{sr, sw}, sftp.WithServerWorkingDirectory(home))

	if err != nil {
		t.Fatal(err)
	}

	go srv.Serve()

	c, err := sftp.NewClientPipe(cr, cw)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		srv.Close()
		cr.Close()
		c.Close()
	})

	return c, home
}

func TestRemoteReplaceCreatesBacksUpAndDetectsChanges(t *testing.T) {
	c, home := sftpPair(t)
	files := Remote(c)

	got, err := files.Read()

	if err != nil || got != nil {
		t.Fatalf("missing file: %q, %v", got, err)
	}

	if err := files.Replace(nil, []byte("first\n")); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(home, sshDir, authorizedName)

	if b, _ := os.ReadFile(path); string(b) != "first\n" {
		t.Fatalf("written = %q", b)
	}

	if err := files.Replace([]byte("first\n"), []byte("second\n")); err != nil {
		t.Fatal(err)
	}

	if b, _ := os.ReadFile(path + backupSuffix); string(b) != "first\n" {
		t.Fatalf("backup = %q", b)
	}

	if _, err := os.Stat(path + tmpSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the temporary file was left behind: %v", err)
	}

	os.WriteFile(path, []byte("edited by someone else\n"), 0o600)

	if err := files.Replace([]byte("second\n"), []byte("third\n")); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}

	if b, _ := os.ReadFile(path); string(b) != "edited by someone else\n" {
		t.Fatalf("a concurrent edit was overwritten: %q", b)
	}
}

func TestRemoteRefusesHugeFiles(t *testing.T) {
	c, home := sftpPair(t)
	dir := filepath.Join(home, sshDir)
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, authorizedName), make([]byte, maxFileSize+1), 0o600)

	if _, err := Remote(c).Read(); !errors.Is(err, ErrTooBig) {
		t.Fatalf("huge file: err = %v", err)
	}
}

func TestRemoteRefusesLinks(t *testing.T) {
	c, home := sftpPair(t)
	files := Remote(c)
	dir := filepath.Join(home, sshDir)
	os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, authorizedName)
	target := filepath.Join(home, "elsewhere")
	os.WriteFile(target, []byte("x\n"), 0o600)

	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := files.Read(); !errors.Is(err, ErrLink) {
		t.Fatalf("link: err = %v", err)
	}
}
