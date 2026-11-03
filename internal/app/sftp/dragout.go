package sftp

import (
	"errors"
	"image"
	"io"
	"path"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/platform/dragdrop"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/transfer"
)

const (
	dragOutThreshold = 10
	dragOutMaxFiles  = 10_000
)

var errTooManyFiles = errors.New("the folder has more than 10,000 items: download it instead")

type dragTrack struct {
	start f32.Point
	down  bool
	fired bool
}

func (p *Panel) trackDrag(a core.Host, gtx C, c *sftpConn, cwd string, e sshx.Entry, size image.Point) {
	if p.drags == nil {
		p.drags = map[string]*dragTrack{}
	}

	t := p.drags[e.Name]

	if t == nil {
		t = &dragTrack{}
		p.drags[e.Name] = t
	}

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: t, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})

		if !ok {
			break
		}

		pe, ok := ev.(pointer.Event)

		if !ok {
			continue
		}

		switch pe.Kind {
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				t.start, t.down, t.fired = pe.Position, true, false
			}
		case pointer.Drag:
			d := pe.Position.Sub(t.start)

			if t.down && !t.fired && d.X*d.X+d.Y*d.Y > float32(gtx.Dp(dragOutThreshold)*gtx.Dp(dragOutThreshold)) {
				t.fired = true
				p.dragOut(a, c, cwd, e)
			}
		case pointer.Release, pointer.Cancel:
			t.down = false
		}
	}

	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, t)
}

func (p *Panel) dragOut(a core.Host, c *sftpConn, cwd string, e sshx.Entry) {
	client := c.sftp()

	if client == nil {
		return
	}

	full := path.Join(cwd, e.Name)
	root := transfer.SafeName(e.Name)

	if !e.IsDir {
		a.DragOut([]dragdrop.VirtualFile{remoteFile(client, full, root, e.Size, e.ModTime)})
		return
	}

	go func() {
		files, err := remoteTree(client, full, root)

		if err != nil {
			a.Toasts().Error("Couldn't drag the folder out", err.Error())
			return
		}

		a.DragOut(files)
	}()
}

func remoteFile(client *sshx.SFTP, remote, name string, size int64, mod time.Time) dragdrop.VirtualFile {
	return dragdrop.VirtualFile{
		Name:    name,
		Size:    size,
		ModTime: mod,
		Open:    func() (io.ReadSeekCloser, error) { return client.Open(remote) },
	}
}

func remoteTree(client *sshx.SFTP, remote, root string) ([]dragdrop.VirtualFile, error) {
	files := []dragdrop.VirtualFile{{Name: root, Dir: true}}
	w := client.Walk(remote)

	for w.Step() {
		if err := w.Err(); err != nil {
			return nil, err
		}

		if w.Path() == remote {
			continue
		}

		if len(files) >= dragOutMaxFiles {
			return nil, errTooManyFiles
		}

		rel, ok := strings.CutPrefix(w.Path(), remote+"/")

		if !ok {
			continue
		}

		parts := strings.Split(rel, "/")

		for i := range parts {
			parts[i] = transfer.SafeName(parts[i])
		}

		name := root + `\` + strings.Join(parts, `\`)
		fi := w.Stat()

		switch {
		case fi.IsDir():
			files = append(files, dragdrop.VirtualFile{Name: name, Dir: true, ModTime: fi.ModTime()})
		case fi.Mode().IsRegular():
			files = append(files, remoteFile(client, w.Path(), name, fi.Size(), fi.ModTime()))
		}
	}

	return files, nil
}
