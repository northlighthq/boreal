package remotefs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pkg/sftp"
)

func pair(t *testing.T) (FS, string) {
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

	return New(c), home
}

func leftovers(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tmpPrefix) {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestListPutsFoldersFirstAndCaps(t *testing.T) {
	f, home := pair(t)
	os.Mkdir(filepath.Join(home, "zeta"), 0o755)
	os.WriteFile(filepath.Join(home, "Alpha.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(home, "beta.txt"), []byte("bb"), 0o644)

	_, entries, truncated, err := f.List("", 0)

	if err != nil || truncated {
		t.Fatalf("list: %v %v", truncated, err)
	}

	var names []string

	for _, e := range entries {
		names = append(names, e.Name+":"+e.Type)
	}

	if got := strings.Join(names, ","); got != "zeta:folder,Alpha.txt:file,beta.txt:file" {
		t.Fatalf("entries = %s", got)
	}

	if _, entries, truncated, _ := f.List(".", 2); len(entries) != 2 || !truncated {
		t.Fatalf("capped list = %d %v", len(entries), truncated)
	}

	if _, _, _, err := f.List("Alpha.txt", 0); !errors.Is(err, ErrNotDir) {
		t.Fatalf("listing a file: %v", err)
	}
}

func TestReadWindowsAndTruncates(t *testing.T) {
	f, home := pair(t)
	os.WriteFile(filepath.Join(home, "log.txt"), []byte("0123456789"), 0o644)

	got, err := f.Read("log.txt", 2, 4)

	if err != nil || string(got.Data) != "2345" || !got.Truncated || got.Size != 10 {
		t.Fatalf("read = %+v, %v", got, err)
	}

	if got, _ := f.Read("log.txt", 0, 0); string(got.Data) != "0123456789" || got.Truncated {
		t.Fatalf("whole read = %+v", got)
	}

	os.Mkdir(filepath.Join(home, "dir"), 0o755)

	if _, err := f.Read("dir", 0, 0); !errors.Is(err, ErrIsDir) {
		t.Fatalf("reading a folder: %v", err)
	}
}

func TestWriteCreatesAndOnlyOverwritesWhenAsked(t *testing.T) {
	f, home := pair(t)
	p := filepath.Join(home, "app.conf")

	w, err := f.Write("app.conf", []byte("one\n"), false)

	if err != nil || !w.Created || w.Bytes != 4 {
		t.Fatalf("create = %+v, %v", w, err)
	}

	if _, err := f.Write("app.conf", []byte("two\n"), false); !errors.Is(err, ErrExists) {
		t.Fatalf("overwrite without asking: %v", err)
	}

	if b, _ := os.ReadFile(p); string(b) != "one\n" {
		t.Fatalf("file changed without overwrite: %q", b)
	}

	if w, err := f.Write("app.conf", []byte("two\n"), true); err != nil || w.Created {
		t.Fatalf("overwrite = %+v, %v", w, err)
	}

	if b, _ := os.ReadFile(p); string(b) != "two\n" {
		t.Fatalf("after overwrite = %q", b)
	}

	leftovers(t, home)
}

func TestWriteKeepsThePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows doesn't keep Unix permissions")
	}

	f, home := pair(t)
	p := filepath.Join(home, "secret.env")
	os.WriteFile(p, []byte("A=1\n"), 0o600)

	if _, err := f.Write("secret.env", []byte("A=2\n"), true); err != nil {
		t.Fatal(err)
	}

	if info, _ := os.Stat(p); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteRefusesSignInFiles(t *testing.T) {
	f, home := pair(t)
	os.Mkdir(filepath.Join(home, ".ssh"), 0o700)
	os.Mkdir(filepath.Join(home, "etc"), 0o755)

	for _, p := range []string{".ssh/authorized_keys", ".ssh/config", ".SSH/rc", "etc/authorized_keys2", "etc/../.ssh/x", "./.ssh"} {
		if _, err := f.Write(p, []byte("ssh-ed25519 AAAA attacker\n"), true); !errors.Is(err, ErrProtected) {
			t.Fatalf("%s: err = %v, want ErrProtected", p, err)
		}
	}

	if entries, _ := os.ReadDir(filepath.Join(home, ".ssh")); len(entries) != 0 {
		t.Fatalf("something was written in .ssh: %v", entries)
	}
}

func TestWriteRefusesLinksIntoSSH(t *testing.T) {
	f, home := pair(t)
	os.Mkdir(filepath.Join(home, ".ssh"), 0o700)

	if err := os.Symlink(filepath.Join(home, ".ssh"), filepath.Join(home, "keys")); err != nil {
		t.Skip("symlinks aren't available here:", err)
	}

	os.Mkdir(filepath.Join(home, "sub"), 0o755)
	os.Symlink("../.ssh", filepath.Join(home, "sub", "rel"))
	os.Symlink("sub/rel", filepath.Join(home, "chain"))
	os.Symlink("loop-b", filepath.Join(home, "loop-a"))
	os.Symlink("loop-a", filepath.Join(home, "loop-b"))

	for _, p := range []string{"keys/authorized", "sub/rel/config", "chain/rc", "sub/../chain/x"} {
		if _, err := f.Write(p, []byte("x"), true); !errors.Is(err, ErrProtected) {
			t.Fatalf("write through a link to .ssh (%s): %v", p, err)
		}
	}

	if _, err := f.Write("loop-a/x", []byte("x"), true); !errors.Is(err, errLoop) {
		t.Fatalf("link loop: %v", err)
	}

	if entries, _ := os.ReadDir(filepath.Join(home, ".ssh")); len(entries) != 0 {
		t.Fatalf("something was written in .ssh: %v", entries)
	}

	if w, err := f.Write("sub/ok.txt", []byte("fine"), false); err != nil || w.Path != filepath.ToSlash(filepath.Join(home, "sub", "ok.txt")) {
		t.Fatalf("normal write = %+v, %v", w, err)
	}

	os.WriteFile(filepath.Join(home, "real.txt"), []byte("real"), 0o644)
	os.Symlink(filepath.Join(home, "real.txt"), filepath.Join(home, "alias.txt"))

	if _, err := f.Write("alias.txt", []byte("x"), true); !errors.Is(err, ErrLink) {
		t.Fatalf("overwriting a link: %v", err)
	}
}

func TestWriteRejectsBadTargets(t *testing.T) {
	f, home := pair(t)
	os.Mkdir(filepath.Join(home, "dir"), 0o755)

	cases := map[string]error{
		"":                ErrEmptyPath,
		"dir":             ErrIsDir,
		"missing/app.txt": ErrNoParent,
	}

	for p, want := range cases {
		if _, err := f.Write(p, []byte("x"), true); !errors.Is(err, want) {
			t.Fatalf("%q: err = %v, want %v", p, err, want)
		}
	}

	if _, err := f.Write("big.bin", bytes.Repeat([]byte("a"), MaxWrite+1), true); !errors.Is(err, ErrTooBig) {
		t.Fatalf("too big: %v", err)
	}

	leftovers(t, home)
}
