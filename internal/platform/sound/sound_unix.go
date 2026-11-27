//go:build (linux && !android) || darwin

package sound

import (
	"os"
	"os/exec"
	"sync"
)

type player struct {
	cmd  string
	args []string
	file string
}

var pick = sync.OnceValue(func() *player {
	for i, p := range candidates {
		if _, err := exec.LookPath(p.cmd); err != nil {
			continue
		}

		if p.file != "" {
			if _, err := os.Stat(p.file); err != nil {
				continue
			}
		}

		return &candidates[i]
	}

	return nil
})

var Available = pick() != nil

func Play() {
	p := pick()

	if p == nil {
		return
	}

	args := p.args

	if p.file != "" {
		args = append(args[:len(args):len(args)], p.file)
	}

	cmd := exec.Command(p.cmd, args...)

	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
