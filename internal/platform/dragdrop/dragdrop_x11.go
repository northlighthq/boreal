//go:build linux && !android && !nox11 && cgo

package dragdrop

/*
#cgo pkg-config: x11

int borealX11Setup(unsigned long target);
void borealX11Loop(void);
*/
import "C"

import (
	"strings"
	"sync"
)

var x11 struct {
	sync.Mutex
	once sync.Once
	drop func([]string)
}

func EnableX11(window uintptr, onDrop func(paths []string), onHover func(over bool)) {
	if window == 0 {
		return
	}

	x11.once.Do(func() {
		setHover(onHover)
		x11.Lock()
		x11.drop = onDrop
		x11.Unlock()

		if C.borealX11Setup(C.ulong(window)) != 0 {
			return
		}

		go C.borealX11Loop()
	})
}

//export borealX11Drop
func borealX11Drop(data *C.char, length C.int) {
	paths := parseURIList(strings.NewReader(C.GoStringN(data, length)))

	x11.Lock()
	drop := x11.drop
	x11.Unlock()

	if drop != nil && len(paths) > 0 {
		go drop(paths)
	}
}
