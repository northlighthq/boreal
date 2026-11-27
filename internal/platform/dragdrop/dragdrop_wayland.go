//go:build linux && !android && !nowayland && cgo

package dragdrop

/*
#cgo pkg-config: wayland-client

#include <stdint.h>

int borealWaylandSetup(uintptr_t display, uintptr_t surface);
void borealWaylandDispatch(uintptr_t display);
void borealWaylandReceive(uintptr_t display, uintptr_t offer, int fd);
void borealWaylandFinish(uintptr_t display, uintptr_t offer);
*/
import "C"

import (
	"bufio"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"unsafe"
)

const maxURIList = 1 << 20

var wayland struct {
	sync.Mutex
	once    sync.Once
	display uintptr
	drop    func([]string)
	hover   func(bool)
}

// EnableWayland receives files dropped on the Wayland surface through a data device of
// Boreal's own, since Gio's ignores drops.
func EnableWayland(display, surface unsafe.Pointer, onDrop func(paths []string), onHover func(over bool)) {
	if display == nil || surface == nil {
		return
	}

	wayland.once.Do(func() {
		d := uintptr(display)

		wayland.Lock()
		wayland.display, wayland.drop, wayland.hover = d, onDrop, onHover
		wayland.Unlock()

		if C.borealWaylandSetup(C.uintptr_t(d), C.uintptr_t(uintptr(surface))) != 0 {
			return
		}

		go C.borealWaylandDispatch(C.uintptr_t(d))
	})
}

//export borealDragHover
func borealDragHover(over C.int) {
	wayland.Lock()
	hover := wayland.hover
	wayland.Unlock()

	if hover != nil {
		hover(over != 0)
	}
}

//export borealWaylandDrop
func borealWaylandDrop(offer C.uintptr_t) {
	r, w, err := os.Pipe()

	wayland.Lock()
	display, drop := wayland.display, wayland.drop
	wayland.Unlock()

	if err != nil {
		C.borealWaylandFinish(C.uintptr_t(display), offer)
		return
	}

	C.borealWaylandReceive(C.uintptr_t(display), offer, C.int(w.Fd()))
	w.Close()

	go func() {
		defer C.borealWaylandFinish(C.uintptr_t(display), offer)
		defer r.Close()

		paths := parseURIList(io.LimitReader(r, maxURIList))

		if drop != nil && len(paths) > 0 {
			drop(paths)
		}
	}()
}

// parseURIList reads text/uri-list (RFC 2483) and keeps the local files.
func parseURIList(r io.Reader) []string {
	var paths []string
	sc := bufio.NewScanner(r)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		u, err := url.Parse(line)

		if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.Path == "" {
			continue
		}

		paths = append(paths, u.Path)
	}

	return paths
}
