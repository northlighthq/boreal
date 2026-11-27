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
	handler func(bool)
}

func setHover(handler func(bool)) {
	hovering.Lock()
	hovering.handler = handler
	hovering.Unlock()
}

//export borealDragHover
func borealDragHover(over C.int) {
	hovering.Lock()
	handler := hovering.handler
	hovering.Unlock()

	if handler != nil {
		handler(over != 0)
	}
}

func parseURIList(reader io.Reader) []string {
	var paths []string
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimRight(scanner.Text(), "\x00"))

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		link, err := url.Parse(line)

		if err != nil || link.Scheme != "file" || (link.Host != "" && link.Host != "localhost") || link.Path == "" {
			continue
		}

		paths = append(paths, link.Path)
	}

	return paths
}
