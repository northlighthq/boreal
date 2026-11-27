//go:build (darwin && cgo) || (linux && !android && cgo)

package dragdrop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const windowsSeparator = `\`

var errNoFiles = errors.New("dragdrop: nothing to drag")

func writeTree(files []VirtualFile, dst string) error {
	if len(files) == 0 {
		return errNoFiles
	}

	root := files[0].Name

	for _, file := range files {
		relative, ok := strings.CutPrefix(file.Name, root)

		if !ok {
			continue
		}

		relative = strings.TrimPrefix(strings.ReplaceAll(relative, windowsSeparator, "/"), "/")
		target := filepath.Join(dst, filepath.FromSlash(relative))

		if inside, err := filepath.Rel(dst, target); err != nil || inside == ".." || strings.HasPrefix(inside, "../") {
			continue
		}

		if file.Dir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("dragdrop: create folder %s: %w", relative, err)
			}

			continue
		}

		if err := writeFile(file, target); err != nil {
			return fmt.Errorf("dragdrop: download %s: %w", filepath.Base(target), err)
		}
	}

	return nil
}

func writeFile(file VirtualFile, target string) error {
	if file.Open == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	remote, err := file.Open()

	if err != nil {
		return err
	}

	defer remote.Close()

	local, err := os.Create(target)

	if err != nil {
		return err
	}

	_, err = io.Copy(local, remote)

	if closeErr := local.Close(); err == nil {
		err = closeErr
	}

	if err == nil && !file.ModTime.IsZero() {
		os.Chtimes(target, file.ModTime, file.ModTime)
	}

	return err
}
