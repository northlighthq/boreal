//go:build !windows

package urlhandler

func Registered() bool {
	return false
}

func Set(enabled bool) error {
	return nil
}
