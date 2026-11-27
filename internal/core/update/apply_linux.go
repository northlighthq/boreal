//go:build linux && !android

package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

const stagedSuffix = ".new"

var errNotAppImage = errors.New("update: only the AppImage can update itself")

var appImageArch = map[string]string{"amd64": "x86_64", "arm64": "aarch64"}

func appImage() string {
	return os.Getenv("APPIMAGE")
}

func SelfUpdate() bool {
	return appImage() != "" && appImageArch[runtime.GOARCH] != ""
}

func assetFor(version string, k Kind) string {
	arch := appImageArch[runtime.GOARCH]

	if appImage() == "" || arch == "" {
		return ""
	}

	return "boreal-" + version + "-linux-" + arch + ".AppImage"
}

func Detect() Kind {
	return Portable
}

func Prepare(path string, k Kind) error {
	target := appImage()

	if target == "" {
		return errNotAppImage
	}

	download, err := os.Open(path)

	if err != nil {
		return fmt.Errorf("update: open download: %w", err)
	}

	defer download.Close()

	stagedPath := target + stagedSuffix
	staged, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)

	if err != nil {
		return fmt.Errorf("update: stage: %w", err)
	}

	_, err = io.Copy(staged, io.LimitReader(download, maxAsset))

	if closeErr := staged.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		os.Remove(stagedPath)
		return fmt.Errorf("update: stage: %w", err)
	}

	return nil
}

func Launch(path string, k Kind) error {
	target := appImage()

	if target == "" {
		return errNotAppImage
	}

	if err := os.Rename(target+stagedSuffix, target); err != nil {
		return fmt.Errorf("update: swap: %w", err)
	}

	return exec.Command(target).Start()
}

func Cleanup(dir string) {
	if target := appImage(); target != "" {
		os.Remove(target + stagedSuffix)
	}

	os.RemoveAll(dir)
}
