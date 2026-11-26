//go:build (linux && !android) || darwin

package sound

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
)

type player struct {
	cmd  string
	args []string
}

var candidates = func() []player {
	if runtime.GOOS == "darwin" {
		return []player{{"afplay", []string{"/System/Library/Sounds/Glass.aiff"}}}
	}

	return []player{
		{"canberra-gtk-play", []string{"-i", "bell", "-d", "Boreal"}},
		{"paplay", []string{"/usr/share/sounds/freedesktop/stereo/bell.oga"}},
		{"pw-play", []string{"/usr/share/sounds/freedesktop/stereo/bell.oga"}},
		{"aplay", []string{"-q", "/usr/share/sounds/alsa/Front_Center.wav"}},
	}
}()

var pick = sync.OnceValue(func() *player {
	for i, p := range candidates {
		if _, err := exec.LookPath(p.cmd); err != nil {
			continue
		}

		if p.cmd != "canberra-gtk-play" {
			if _, err := os.Stat(p.args[len(p.args)-1]); err != nil {
				continue
			}
		}

		return &candidates[i]
	}

	return nil
})

// Available reports whether a sound player was found on this system.
var Available = pick() != nil

func Play() {
	p := pick()

	if p == nil {
		return
	}

	cmd := exec.Command(p.cmd, p.args...)

	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
