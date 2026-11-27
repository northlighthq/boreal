//go:build linux && !android && cgo

package dragdrop

import "C"

import (
	"bufio"
	"io"
	"net/url"
	"strings"
	"sync"
)

const maxURIList = 1 << 20

var hovering struct {
	sync.Mutex
	fn func(bool)
}

func setHover(fn func(bool)) {
	hovering.Lock()
	hovering.fn = fn
	hovering.Unlock()
}

//export borealDragHover
func borealDragHover(over C.int) {
	hovering.Lock()
	fn := hovering.fn
	hovering.Unlock()

	if fn != nil {
		fn(over != 0)
	}
}

func parseURIList(r io.Reader) []string {
	var paths []string
	sc := bufio.NewScanner(r)

	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimRight(sc.Text(), "\x00"))

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
