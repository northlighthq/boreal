//go:build linux && !android && !nowayland && cgo

package dragdrop

/*
#cgo pkg-config: wayland-client

#include <stdint.h>

int borealWaylandSetup(uintptr_t display, uintptr_t surface);
void borealWaylandDispatch(uintptr_t display);
void borealWaylandReceive(uintptr_t display, uintptr_t offer, int fd);
void borealWaylandFinish(uintptr_t display, uintptr_t offer);
int borealWaylandStartDrag(void);
*/
import "C"

import (
	"io"
	"os"
	"sync"
	"unsafe"
)

var wayland struct {
	sync.Mutex
	once    sync.Once
	display uintptr
	drop    func([]string)
}

func EnableWayland(display, surface unsafe.Pointer, onDrop func(paths []string), onHover func(over bool)) {
	if display == nil || surface == nil {
		return
	}

	wayland.once.Do(func() {
		displayPtr := uintptr(display)

		wayland.Lock()
		wayland.display, wayland.drop = displayPtr, onDrop
		wayland.Unlock()
		setHover(onHover)

		if C.borealWaylandSetup(C.uintptr_t(displayPtr), C.uintptr_t(uintptr(surface))) != 0 {
			return
		}

		go C.borealWaylandDispatch(C.uintptr_t(displayPtr))
		setBackend(startWaylandDrag)
	})
}

func startWaylandDrag() error {
	if C.borealWaylandStartDrag() != 0 {
		return ErrReleased
	}

	return nil
}

//export borealWaylandSend
func borealWaylandSend(fd C.int32_t) {
	pipe := os.NewFile(uintptr(fd), "boreal-drag")

	go func() {
		defer pipe.Close()

		uris, err := stagedURIs()

		if err != nil {
			return
		}

		io.WriteString(pipe, uris)
	}()
}

//export borealWaylandDrop
func borealWaylandDrop(offer C.uintptr_t) {
	reader, writer, err := os.Pipe()

	wayland.Lock()
	display, drop := wayland.display, wayland.drop
	wayland.Unlock()

	if err != nil {
		C.borealWaylandFinish(C.uintptr_t(display), offer)
		return
	}

	C.borealWaylandReceive(C.uintptr_t(display), offer, C.int(writer.Fd()))
	writer.Close()

	go func() {
		defer C.borealWaylandFinish(C.uintptr_t(display), offer)
		defer reader.Close()

		paths := parseURIList(io.LimitReader(reader, maxURIList))

		if drop != nil && len(paths) > 0 {
			drop(paths)
		}
	}()
}
