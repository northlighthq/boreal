package update

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	exeName       = "boreal.exe"
	uninstallName = "unins000.exe"
)

var ErrNoBinary = errors.New("update: the archive has no boreal.exe")

func Detect() Kind {
	exe, err := os.Executable()

	if err != nil {
		return Portable
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), uninstallName)); err == nil {
		return Installer
	}

	return Portable
}

func Prepare(path string, k Kind) error {
	if k == Installer {
		return nil
	}

	exe, err := os.Executable()

	if err != nil {
		return err
	}

	zr, err := zip.OpenReader(path)

	if err != nil {
		return err
	}

	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != exeName {
			continue
		}

		return extract(f, exe+".new")
	}

	return ErrNoBinary
}

func extract(f *zip.File, dst string) error {
	in, err := f.Open()

	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)

	if err != nil {
		return err
	}

	_, err = io.Copy(out, io.LimitReader(in, maxAsset))

	if cerr := out.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		os.Remove(dst)
	}

	return err
}

func Launch(path string, k Kind) error {
	if k == Installer {
		return exec.Command(path, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/UPDATE").Start()
	}

	exe, err := os.Executable()

	if err != nil {
		return err
	}

	old := exe + ".old"
	os.Remove(old)

	if err := os.Rename(exe, old); err != nil {
		return err
	}

	if err := os.Rename(exe+".new", exe); err != nil {
		os.Rename(old, exe)
		return err
	}

	return exec.Command(exe).Start()
}

func Cleanup(dir string) {
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
		os.Remove(exe + ".new")
	}

	os.RemoveAll(dir)
}
