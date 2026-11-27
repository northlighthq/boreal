package dragdrop

import (
	"errors"
	"io"
	"sync"
	"time"
)

var ErrReleased = errors.New("dragdrop: the mouse button was released before the drag started")

var failure struct {
	sync.Mutex
	handler func(error)
}

func OnFailure(handler func(error)) {
	failure.Lock()
	failure.handler = handler
	failure.Unlock()
}

func reportFailure(err error) {
	failure.Lock()
	handler := failure.handler
	failure.Unlock()

	if handler != nil {
		handler(err)
	}
}

type VirtualFile struct {
	Name    string
	Size    int64
	ModTime time.Time
	Dir     bool
	Open    func() (io.ReadSeekCloser, error)
}
