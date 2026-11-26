//go:build !windows && !(linux && !android)

package update

// SelfUpdate reports whether this build can download and install its own updates.
// macOS builds ship as a disk image, so the update comes from the release page.
func SelfUpdate() bool { return false }

func assetFor(version string, k Kind) string { return "" }

func Detect() Kind { return Portable }

func Prepare(path string, k Kind) error { return ErrMissing }

func Launch(path string, k Kind) error { return ErrMissing }

func Cleanup(dir string) {}
