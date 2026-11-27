//go:build !windows && !(linux && !android) && !darwin

package sound

const Available = false

func Play() {}
