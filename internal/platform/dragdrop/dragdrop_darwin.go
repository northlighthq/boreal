//go:build darwin && cgo

package dragdrop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit

#include <stdint.h>
#include <stdlib.h>

void borealEnableDrop(uintptr_t view);
int borealDragPromise(uintptr_t view, uintptr_t handle, const char *name, int dir);
void borealReleasePromise(uintptr_t handle);
*/
import "C"

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

var handlers struct {
	sync.Mutex
	drop  func([]string)
	hover func(bool)
	view  uintptr
}

var promises struct {
	sync.Mutex
	next  uintptr
	files map[uintptr][]VirtualFile
}

var errNoFiles = errors.New("dragdrop: nothing to drag")

// Enable lets Finder drop files on the window. Only one window is supported, like on Windows.
func Enable(view uintptr, onDrop func(paths []string), onHover func(over bool)) {
	if view == 0 {
		return
	}

	handlers.Lock()
	handlers.drop, handlers.hover, handlers.view = onDrop, onHover, view
	handlers.Unlock()

	C.borealEnableDrop(C.uintptr_t(view))
}

//export borealDrop
func borealDrop(joined *C.char) {
	paths := strings.Split(strings.TrimSuffix(C.GoString(joined), "\n"), "\n")

	handlers.Lock()
	drop := handlers.drop
	handlers.Unlock()

	if drop != nil && len(paths) > 0 && paths[0] != "" {
		go drop(paths)
	}
}

//export borealDragHover
func borealDragHover(over C.int) {
	handlers.Lock()
	hover := handlers.hover
	handlers.Unlock()

	if hover != nil {
		hover(over != 0)
	}
}

func DragOut(paths []string) error {
	return errors.ErrUnsupported
}

// DragVirtual drags remote files out as a file promise: Finder picks the folder and
// Boreal writes the files there once the drop lands. files[0] is the root, file or folder,
// and the other names are paths under it.
func DragVirtual(files []VirtualFile) error {
	handlers.Lock()
	view := handlers.view
	handlers.Unlock()

	if view == 0 {
		return errors.ErrUnsupported
	}

	if len(files) == 0 {
		return errNoFiles
	}

	promises.Lock()

	if promises.files == nil {
		promises.files = map[uintptr][]VirtualFile{}
	}

	promises.next++
	h := promises.next
	promises.files[h] = files
	promises.Unlock()

	root := files[0]
	name := C.CString(root.Name)
	defer C.free(unsafe.Pointer(name))

	dir := 0

	if root.Dir {
		dir = 1
	}

	if C.borealDragPromise(C.uintptr_t(view), C.uintptr_t(h), name, C.int(dir)) != 0 {
		release(h)
		return ErrReleased
	}

	return nil
}

func release(h uintptr) {
	promises.Lock()
	delete(promises.files, h)
	promises.Unlock()
	C.borealReleasePromise(C.uintptr_t(h))
}

//export borealPromiseDone
func borealPromiseDone(h C.uintptr_t) {
	go release(uintptr(h))
}

// borealWritePromise writes the promised files to dst, the path Finder chose for files[0].
// It returns NULL on success, or an error message the caller frees.
//
//export borealWritePromise
func borealWritePromise(h C.uintptr_t, dst *C.char) *C.char {
	promises.Lock()
	files := promises.files[uintptr(h)]
	promises.Unlock()

	defer func() { go release(uintptr(h)) }()

	if err := writeTree(files, C.GoString(dst)); err != nil {
		return C.CString(err.Error())
	}

	return nil
}

func writeTree(files []VirtualFile, dst string) error {
	if len(files) == 0 {
		return errNoFiles
	}

	root := files[0].Name

	for _, f := range files {
		// Names come Windows-style: root\sub\file.
		rel, ok := strings.CutPrefix(f.Name, root)

		if !ok {
			continue
		}

		rel = strings.TrimPrefix(strings.ReplaceAll(rel, `\`, "/"), "/")
		target := filepath.Join(dst, filepath.FromSlash(rel))

		if inside, err := filepath.Rel(dst, target); err != nil || inside == ".." || strings.HasPrefix(inside, "../") {
			continue
		}

		if f.Dir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}

			continue
		}

		if err := writeFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

func writeFile(f VirtualFile, target string) error {
	if f.Open == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	in, err := f.Open()

	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.Create(target)

	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)

	if cerr := out.Close(); err == nil {
		err = cerr
	}

	if err == nil && !f.ModTime.IsZero() {
		os.Chtimes(target, f.ModTime, f.ModTime)
	}

	return err
}
