package sftp

import (
	"image"
	"os"
	"path"
	"path/filepath"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/transfer"
)

const (
	dragOutThreshold = 10
	dragOutCleanup   = 15 * time.Minute
)

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
	dir, err := os.MkdirTemp("", "boreal-drag-")

	if err != nil {
		a.Toasts().Error("Couldn't drag the file out", err.Error())
		return
	}

	local := filepath.Join(dir, transfer.SafeName(e.Name))
	p.enqueue(a, c, &transfer.Job{Kind: transfer.Download, Name: e.Name, Remote: path.Join(cwd, e.Name), Local: local}, "", func(*transfer.Job) {
		a.DragOut([]string{local})
		time.AfterFunc(dragOutCleanup, func() { os.RemoveAll(dir) })
	})
}
