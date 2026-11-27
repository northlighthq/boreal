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
	once sync.Once
	drop func([]string)
}

func EnableX11(window uintptr, onDrop func(paths []string), onHover func(over bool)) {
	if window == 0 {
		return
	}

	x11.once.Do(func() {
		setHover(onHover)
		x11.drop = onDrop

		if C.borealX11Setup(C.ulong(window)) != 0 {
			return
		}

		go C.borealX11Loop()
	})
}

//export borealX11Drop
func borealX11Drop(data *C.char, n C.int) {
	paths := parseURIList(strings.NewReader(C.GoStringN(data, n)))

	if x11.drop != nil && len(paths) > 0 {
		go x11.drop(paths)
	}
}
