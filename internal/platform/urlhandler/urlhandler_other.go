//go:build !windows && !(linux && !android) && !(darwin && cgo)

package urlhandler

const (
	Supported = false
	Hint      = ""
)

func Registered() bool {
	return false
}

func Set(enabled bool) error {
	return nil
}
