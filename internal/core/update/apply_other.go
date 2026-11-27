//go:build !windows && !(linux && !android)

package update

func SelfUpdate() bool { return false }

func assetFor(version string, k Kind) string { return "" }

func Detect() Kind { return Portable }

func Prepare(path string, k Kind) error { return ErrMissing }

func Launch(path string, k Kind) error { return ErrMissing }

func Cleanup(dir string) {}
