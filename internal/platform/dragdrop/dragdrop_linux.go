//go:build linux && !android && cgo

package dragdrop

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	stagingPattern = "boreal-drag-"
	uriSeparator   = "\r\n"
)

var dragging struct {
	sync.Mutex
	files   []VirtualFile
	staging string
	uris    string
	err     error
	staged  bool
}

var backend struct {
	sync.Mutex
	start func() error
}

func Enable(hwnd uintptr, onDrop func(paths []string), onHover func(over bool)) {}

func DragOut(paths []string) error {
	return errors.ErrUnsupported
}

func DragVirtual(files []VirtualFile) error {
	if len(files) == 0 {
		return errNoFiles
	}

	backend.Lock()
	start := backend.start
	backend.Unlock()

	if start == nil {
		return errors.ErrUnsupported
	}

	dragging.Lock()

	if dragging.staging != "" {
		os.RemoveAll(dragging.staging)
	}

	dragging.files, dragging.staging, dragging.uris, dragging.err, dragging.staged = files, "", "", nil, false
	dragging.Unlock()

	return start()
}

func setBackend(start func() error) {
	backend.Lock()
	backend.start = start
	backend.Unlock()
}

func stagedURIs() (string, error) {
	dragging.Lock()
	defer dragging.Unlock()

	if dragging.staged {
		return dragging.uris, dragging.err
	}

	dragging.staged = true
	dragging.uris, dragging.staging, dragging.err = stage(dragging.files)
	return dragging.uris, dragging.err
}

func stage(files []VirtualFile) (string, string, error) {
	if len(files) == 0 {
		return "", "", errNoFiles
	}

	staging, err := os.MkdirTemp("", stagingPattern)

	if err != nil {
		return "", "", err
	}

	root := filepath.Join(staging, filepath.Base(strings.ReplaceAll(files[0].Name, `\`, "/")))

	if err := writeTree(files, root); err != nil {
		os.RemoveAll(staging)
		return "", "", err
	}

	if !files[0].Dir {
		if _, err := os.Stat(root); err != nil {
			os.RemoveAll(staging)
			return "", "", err
		}
	}

	location := url.URL{Scheme: "file", Path: root}
	return location.String() + uriSeparator, staging, nil
}
