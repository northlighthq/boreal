package transfer

import (
	"bytes"
	"io"
	"os"

	"github.com/pkg/sftp"
)

const (
	PartSuffix = ".boreal-part"
	verifySize = 64 << 10
)

func localResume(part string, remote io.ReaderAt, size int64) int64 {
	f, err := os.Open(part)

	if err != nil {
		return 0
	}

	defer f.Close()

	fi, err := f.Stat()

	if err != nil || fi.Size() == 0 || fi.Size() > size {
		return 0
	}

	if !sameTail(f, remote, fi.Size()) {
		return 0
	}

	return fi.Size()
}

func remoteResume(c *sftp.Client, local *os.File, dst string) int64 {
	lfi, err := local.Stat()

	if err != nil {
		return 0
	}

	rfi, err := c.Stat(dst)

	if err != nil || rfi.Size() == 0 || rfi.Size() > lfi.Size() {
		return 0
	}

	f, err := c.Open(dst)

	if err != nil {
		return 0
	}

	defer f.Close()

	if !sameTail(local, f, rfi.Size()) {
		return 0
	}

	return rfi.Size()
}

func sameTail(a, b io.ReaderAt, end int64) bool {
	n := min(end, verifySize)
	x, y := make([]byte, n), make([]byte, n)

	if _, err := io.ReadFull(io.NewSectionReader(a, end-n, n), x); err != nil {
		return false
	}

	if _, err := io.ReadFull(io.NewSectionReader(b, end-n, n), y); err != nil {
		return false
	}

	return bytes.Equal(x, y)
}
