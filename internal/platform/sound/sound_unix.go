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
	for i, candidate := range candidates {
		if _, err := exec.LookPath(candidate.cmd); err != nil {
			continue
		}

		if candidate.file != "" {
			if _, err := os.Stat(candidate.file); err != nil {
				continue
			}
		}

		return &candidates[i]
	}

	return nil
})

var Available = pick() != nil

func Play() {
	chosen := pick()

	if chosen == nil {
		return
	}

	args := chosen.args

	if chosen.file != "" {
		args = append(args[:len(args):len(args)], chosen.file)
	}

	cmd := exec.Command(chosen.cmd, args...)

	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
