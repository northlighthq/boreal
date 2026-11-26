//go:build darwin && cgo

package dragdrop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit

#include <stdint.h>

void borealEnableDrop(uintptr_t view);
*/
import "C"

import (
	"errors"
	"strings"
	"sync"
)

var handlers struct {
	sync.Mutex
	drop  func([]string)
	hover func(bool)
}

// Enable lets Finder drop files on the window. Only one window is supported, like on Windows.
func Enable(view uintptr, onDrop func(paths []string), onHover func(over bool)) {
	if view == 0 {
		return
	}

	handlers.Lock()
	handlers.drop, handlers.hover = onDrop, onHover
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
	return errors.ErrUnsupported
}
