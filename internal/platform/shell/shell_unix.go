//go:build !windows && !android

package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

var known = []Profile{
	{ID: "bash", Name: "Bash", Command: []string{"bash", "-l"}},
	{ID: "zsh", Name: "Zsh", Command: []string{"zsh", "-l"}},
	{ID: "fish", Name: "Fish", Command: []string{"fish", "-l"}},
}

func Detect() []Profile {
	var out []Profile

	for _, p := range known {
		if path, err := exec.LookPath(p.Command[0]); err == nil {
			out = append(out, Profile{ID: p.ID, Name: p.Name, Command: []string{path, "-l"}})
		}
	}

	login := filepath.Base(os.Getenv("SHELL"))

	if i := slices.IndexFunc(out, func(p Profile) bool { return p.ID == login }); i > 0 {
		p := out[i]
		out = append([]Profile{p}, slices.Delete(out, i, i+1)...)
	}

	if len(out) == 0 {
		out = append(out, Profile{ID: "sh", Name: "Shell", Command: []string{"/bin/sh", "-l"}})
	}

	return out
}
