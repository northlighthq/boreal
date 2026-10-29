package sftp

import (
	"path"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/sshx"
)

func (p *Panel) keys(a core.Host, gtx C, c *sftpConn, cwd string, entries []sshx.Entry) {
	names := []key.Name{key.NameF2, key.NameDeleteForward, key.NameReturn, key.NameEnter, key.NameDeleteBackward, key.NameUpArrow, key.NameDownArrow}
	filters := make([]event.Filter, len(names))

	for i, n := range names {
		filters[i] = key.Filter{Focus: p, Name: n}
	}

	for {
		ev, ok := gtx.Event(filters...)

		if !ok {
			break
		}

		e, ok := ev.(key.Event)

		if !ok || e.State != key.Press {
			continue
		}

		sel := -1

		for i, en := range entries {
			if en.Name == p.selected {
				sel = i
			}
		}

		switch e.Name {
		case key.NameUpArrow, key.NameDownArrow:
			if len(entries) == 0 {
				continue
			}

			step := 1

			if e.Name == key.NameUpArrow {
				step = -1
			}

			next := min(max(sel+step, 0), len(entries)-1)

			if sel < 0 {
				next = 0
			}

			p.selected = entries[next].Name
			p.list.ScrollTo(next)
		case key.NameDeleteBackward:
			p.load(a, c, path.Dir(cwd))
		}

		if sel < 0 {
			continue
		}

		en := entries[sel]

		switch e.Name {
		case key.NameF2:
			p.rename(a, c, cwd, en)
		case key.NameDeleteForward:
			p.remove(a, c, cwd, en)
		case key.NameReturn, key.NameEnter:
			if en.IsDir {
				p.load(a, c, path.Join(cwd, en.Name))
			} else {
				go p.downloadFile(a, c, path.Join(cwd, en.Name))
			}
		}
	}
}

func (p *Panel) rename(a core.Host, c *sftpConn, cwd string, e sshx.Entry) {
	f := dialogs.NewForm(a.Kit(), "Rename", "Rename", dialogs.NewField("Name", e.Name, e.Name))
	f.OnOK = func(v []string) bool {
		name := strings.TrimSpace(v[0])

		if name == "" || strings.Contains(name, "/") {
			return false
		}

		if name == e.Name {
			return true
		}

		go func() {
			if err := c.sftp().Rename(path.Join(cwd, e.Name), path.Join(cwd, name)); err != nil {
				a.Toasts().Error("Couldn't rename", err.Error())
				return
			}

			p.load(a, c, cwd)
		}()
		return true
	}
	a.OpenIn(p, f)
}

func (p *Panel) remove(a core.Host, c *sftpConn, cwd string, e sshx.Entry) {
	full := path.Join(cwd, e.Name)
	a.Open(dialogs.NewConfirm(a.Kit(), "Delete from the server?", "“"+e.Name+"” will be permanently deleted. This can't be undone.", "Delete", func() {
		go func() {
			if err := c.sftp().RemoveAll(full); err != nil {
				a.Toasts().Error("Couldn't delete", err.Error())
			}

			p.load(a, c, cwd)
		}()
	}))
}
