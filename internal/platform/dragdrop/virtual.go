package dragdrop

import (
	"errors"
	"io"
	"time"
)

var ErrReleased = errors.New("dragdrop: the mouse button was released before the drag started")

type VirtualFile struct {
	Name    string
	Size    int64
	ModTime time.Time
	Dir     bool
	Open    func() (io.ReadSeekCloser, error)
}
