package mcp

import (
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"

	"github.com/northlighthq/boreal/internal/core/remotefs"
	"github.com/northlighthq/boreal/internal/core/vault"
)

var withFiles = vault.MCPPolicy{Enabled: true, Read: true, Files: true, Folders: []string{"Production"}}
var withWrite = vault.MCPPolicy{Enabled: true, Read: true, Files: true, WriteFiles: true, Folders: []string{"Production"}}

func withHome(t *testing.T, f *fixture) string {
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

	f.view.fs = remotefs.New(c)
	return home
}

func TestFilesNeedTheirOwnSwitches(t *testing.T) {
	f := setup(t, vault.MCPPolicy{Enabled: true, Read: true, Access: true, Execute: true, AllHosts: true})
	withHome(t, f)

	calls := map[string]map[string]any{
		toolListFiles: {"host": "web"},
		toolReadFile:  {"host": "web", "path": "a.txt"},
		toolWriteFile: {"host": "web", "path": "a.txt", "content": "x"},
	}

	for tool, args := range calls {
		if text, isErr := f.call(t, tool, args); !isErr || !strings.Contains(text, "working with files") {
			t.Fatalf("%s without Files: %v %q", tool, isErr, text)
		}
	}

	f.view.policy = withFiles

	if text, isErr := f.call(t, toolWriteFile, map[string]any{"host": "web", "path": "a.txt", "content": "x"}); !isErr || !strings.Contains(text, "writing files") {
		t.Fatalf("write without WriteFiles: %v %q", isErr, text)
	}

	f.view.policy = vault.MCPPolicy{Enabled: true, Read: true, WriteFiles: true, AllHosts: true}

	if _, isErr := f.call(t, toolWriteFile, map[string]any{"host": "web", "path": "a.txt", "content": "x"}); !isErr {
		t.Fatal("WriteFiles alone allowed a write")
	}

	if len(f.view.browse) != 0 {
		t.Fatalf("a host was reached without permission: %v", f.view.browse)
	}
}

func TestFilesOnlyReachSharedHosts(t *testing.T) {
	f := setup(t, withWrite)
	withHome(t, f)

	if text, isErr := f.call(t, toolListFiles, map[string]any{"host": "db-01"}); !isErr || !strings.Contains(text, "no host") {
		t.Fatalf("unshared host: %v %q", isErr, text)
	}

	if len(f.view.browse) != 0 {
		t.Fatal("an unshared host was reached")
	}
}

func TestListReadAndWriteFiles(t *testing.T) {
	f := setup(t, withWrite)
	home := withHome(t, f)
	os.WriteFile(filepath.Join(home, "notes.txt"), []byte("hola\n"), 0o644)
	os.WriteFile(filepath.Join(home, "blob.bin"), []byte{0xff, 0xfe, 0x00, 0x01}, 0o644)

	text, isErr := f.call(t, toolListFiles, map[string]any{"host": "web"})

	if isErr || !strings.Contains(text, `"name":"notes.txt"`) || !strings.Contains(text, `"type":"file"`) {
		t.Fatalf("list = %v %q", isErr, text)
	}

	if text, isErr := f.call(t, toolReadFile, map[string]any{"host": "web", "path": "notes.txt"}); isErr || !strings.Contains(text, `"content":"hola\n"`) || !strings.Contains(text, `"encoding":"utf-8"`) {
		t.Fatalf("read text = %v %q", isErr, text)
	}

	want := base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0x00, 0x01})

	if text, isErr := f.call(t, toolReadFile, map[string]any{"host": "web", "path": "blob.bin"}); isErr || !strings.Contains(text, `"encoding":"base64"`) || !strings.Contains(text, want) {
		t.Fatalf("read binary = %v %q", isErr, text)
	}

	if text, isErr := f.call(t, toolWriteFile, map[string]any{"host": "web", "path": "notes.txt", "content": "adios\n"}); !isErr || !strings.Contains(text, "overwrite") {
		t.Fatalf("overwrite without asking = %v %q", isErr, text)
	}

	if text, isErr := f.call(t, toolWriteFile, map[string]any{"host": "web", "path": "notes.txt", "content": "YWRpb3MK", "encoding": "base64", "overwrite": true}); isErr || !strings.Contains(text, `"bytes":6`) {
		t.Fatalf("overwrite = %v %q", isErr, text)
	}

	if b, _ := os.ReadFile(filepath.Join(home, "notes.txt")); string(b) != "adios\n" {
		t.Fatalf("file = %q", b)
	}

	if _, isErr := f.call(t, toolWriteFile, map[string]any{"host": "web", "path": "x", "content": "!!", "encoding": "base64"}); !isErr {
		t.Fatal("bad base64 was accepted")
	}
}

func TestWriteFileNeverTouchesSSH(t *testing.T) {
	f := setup(t, withWrite)
	home := withHome(t, f)
	os.Mkdir(filepath.Join(home, ".ssh"), 0o700)

	text, isErr := f.call(t, toolWriteFile, map[string]any{"host": "web", "path": ".ssh/authorized_keys", "content": "ssh-ed25519 AAAA attacker", "overwrite": true})

	if !isErr || !strings.Contains(text, "grant_access") {
		t.Fatalf("write to authorized_keys = %v %q", isErr, text)
	}

	if entries, _ := os.ReadDir(filepath.Join(home, ".ssh")); len(entries) != 0 {
		t.Fatal("something was written in .ssh")
	}
}

func TestReadKeepsTextWhenCutMidCharacter(t *testing.T) {
	f := setup(t, withFiles)
	home := withHome(t, f)
	os.WriteFile(filepath.Join(home, "es.txt"), []byte("añadir"), 0o644)

	text, isErr := f.call(t, toolReadFile, map[string]any{"host": "web", "path": "es.txt", "maxBytes": 2})

	if isErr || !strings.Contains(text, `"content":"a"`) || !strings.Contains(text, `"encoding":"utf-8"`) || !strings.Contains(text, `"truncated":true`) {
		t.Fatalf("cut read = %v %q", isErr, text)
	}
}

func TestFilesAreLoggedWithThePath(t *testing.T) {
	f := setup(t, withWrite)
	withHome(t, f)
	f.call(t, toolWriteFile, map[string]any{"host": "web", "path": "app.conf", "content": "x"})

	last := f.log.records[len(f.log.records)-1]

	if last.Tool != toolWriteFile || last.Host != "web" || last.Path != "app.conf" || last.Outcome != OutcomeOK {
		t.Fatalf("record = %+v", last)
	}
}
