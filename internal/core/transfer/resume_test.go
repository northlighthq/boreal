package transfer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalResume(t *testing.T) {
	remote := bytes.Repeat([]byte("0123456789abcdef"), 10_000)
	dir := t.TempDir()
	part := filepath.Join(dir, "file"+PartSuffix)

	cases := []struct {
		name  string
		local []byte
		want  int64
	}{
		{"matching prefix", remote[:70_000], 70_000},
		{"complete", remote, int64(len(remote))},
		{"different bytes", append(append([]byte(nil), remote[:69_999]...), 'X'), 0},
		{"longer than remote", append(append([]byte(nil), remote...), 'X'), 0},
		{"empty", nil, 0},
	}

	for _, c := range cases {
		os.WriteFile(part, c.local, 0o600)

		if got := localResume(part, bytes.NewReader(remote), int64(len(remote))); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}

	if got := localResume(filepath.Join(dir, "missing"), bytes.NewReader(remote), int64(len(remote))); got != 0 {
		t.Errorf("missing part: got %d", got)
	}
}

func TestResumable(t *testing.T) {

	cases := []struct {
		job  *Job
		want bool
	}{
		{&Job{Local: `C:\Users\me\Downloads\a.tar`, HostID: "h"}, true},
		{&Job{Local: `C:\Users\me\Downloads\a.tar`}, false},
		{&Job{HostID: "h"}, false},
		{&Job{Local: filepath.Join(os.TempDir(), "boreal-edit-1", "x"), HostID: "h"}, false},
	}

	for i, c := range cases {
		if got := resumable(c.job); got != c.want {
			t.Errorf("case %d: got %v", i, got)
		}
	}

}
