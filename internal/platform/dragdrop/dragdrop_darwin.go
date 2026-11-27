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
	"fmt"
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
	handle := promises.next
	promises.files[handle] = files
	promises.Unlock()

	root := files[0]
	name := C.CString(root.Name)
	defer C.free(unsafe.Pointer(name))

	dir := 0

	if root.Dir {
		dir = 1
	}

	if C.borealDragPromise(C.uintptr_t(view), C.uintptr_t(handle), name, C.int(dir)) != 0 {
		release(handle)
		return ErrReleased
	}

	return nil
}

func release(handle uintptr) {
	promises.Lock()
	delete(promises.files, handle)
	promises.Unlock()
	C.borealReleasePromise(C.uintptr_t(handle))
}

//export borealPromiseDone
func borealPromiseDone(handle C.uintptr_t) {
	go release(uintptr(handle))
}

//export borealWritePromise
func borealWritePromise(handle C.uintptr_t, dst *C.char) *C.char {
	promises.Lock()
	files := promises.files[uintptr(handle)]
	promises.Unlock()

	defer func() { go release(uintptr(handle)) }()

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

	for _, file := range files {
		relative, ok := strings.CutPrefix(file.Name, root)

		if !ok {
			continue
		}

		relative = strings.TrimPrefix(strings.ReplaceAll(relative, `\`, "/"), "/")
		target := filepath.Join(dst, filepath.FromSlash(relative))

		if inside, err := filepath.Rel(dst, target); err != nil || inside == ".." || strings.HasPrefix(inside, "../") {
			continue
		}

		if file.Dir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("dragdrop: create folder %s: %w", relative, err)
			}

			continue
		}

		if err := writeFile(file, target); err != nil {
			return fmt.Errorf("dragdrop: download %s: %w", filepath.Base(target), err)
		}
	}

	return nil
}

func writeFile(file VirtualFile, target string) error {
	if file.Open == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	remote, err := file.Open()

	if err != nil {
		return err
	}

	defer remote.Close()

	local, err := os.Create(target)

	if err != nil {
		return err
	}

	_, err = io.Copy(local, remote)

	if closeErr := local.Close(); err == nil {
		err = closeErr
	}

	if err == nil && !file.ModTime.IsZero() {
		os.Chtimes(target, file.ModTime, file.ModTime)
	}

	return err
}
