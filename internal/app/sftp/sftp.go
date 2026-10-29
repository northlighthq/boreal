package sftp

import (
	"fmt"
	"image"
	"io"
	"path"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/format"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	sftpMenuOpen = iota
	sftpMenuEdit
	sftpMenuDownload
	sftpMenuCopyPath
	sftpMenuRename
	sftpMenuSep
	sftpMenuDelete
)

type sftpConn struct {
	mu      sync.Mutex
	client  *sshx.SFTP
	owner   *sshx.Client
	cwd     string
	entries []sshx.Entry
	loading bool
	err     error
}

func (c *sftpConn) sftp() *sshx.SFTP {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.client
}

func (c *sftpConn) snapshot() (string, []sshx.Entry, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cwd, c.entries, c.loading, c.err
}

type Panel struct {
	h core.Host

	connsMu sync.Mutex
	conns   map[string]*sftpConn

	up, reload, mkdir, upload, download, follow widget.Clickable
	path                                        widget.Editor
	list                                        kit.ScrollList
	menu                                        kit.Menu
	rows                                        map[string]*kit.Row
	selected                                    string
	pick                                        sshx.Entry
	lastCwd                                     string
	jobRows                                     map[int]*jobRow
	clear                                       widget.Clickable
	drags                                       map[string]*dragTrack
}

func (p *Panel) Title() string     { return "SFTP" }
func (p *Panel) Icon() *icons.Icon { return icons.TransferOutline }
func (p *Panel) Side() core.Side   { return core.Right }
func (p *Panel) Shortcut() string  { return "Alt+6" }
func (p *Panel) Refresh()          {}

func (p *Panel) conn(a core.Host) (*sftpConn, *session.Session) {
	s := a.ActiveSession()

	if s == nil || s.Kind != session.SSH {
		return nil, nil
	}

	client := s.Client()

	if client == nil {
		return nil, s
	}

	p.connsMu.Lock()

	if p.conns == nil {
		p.conns = map[string]*sftpConn{}
	}

	c := p.conns[s.ID]

	if c != nil && c.owner == client {
		p.connsMu.Unlock()
		return c, s
	}

	c = &sftpConn{owner: client, loading: true}
	p.conns[s.ID] = c
	p.connsMu.Unlock()

	go func() {
		sc, err := sshx.OpenSFTP(client)

		c.mu.Lock()
		c.client, c.err = sc, err
		c.loading = false
		c.mu.Unlock()

		if err == nil {
			wd, werr := sc.Getwd()

			if werr != nil || wd == "" {
				wd = "/"
			}

			if _, cwd := s.Terminal().Cwd(); cwd != "" {
				wd = cwd
			}

			p.load(a, c, wd)
		}

		a.Invalidate()
	}()

	return c, s
}

func (p *Panel) connFor(sessID string) *sftpConn {
	p.connsMu.Lock()
	defer p.connsMu.Unlock()

	return p.conns[sessID]
}

func (p *Panel) load(a core.Host, c *sftpConn, dir string) {
	c.mu.Lock()
	c.loading = true
	client := c.client
	c.mu.Unlock()

	go func() {
		entries, err := client.List(dir)

		c.mu.Lock()
		c.loading = false

		if err == nil {
			c.cwd, c.entries, c.err = dir, entries, nil
		} else {
			c.err = err
		}

		c.mu.Unlock()

		if err != nil {
			a.Toasts().Error("SFTP", err.Error())
		}

		a.Invalidate()
	}()
}

func (p *Panel) Actions(gtx C) D {
	return D{}
}

func (p *Panel) Body(gtx C) D {
	a := p.h
	k := a.Kit()
	th := a.Kit().Th
	p.path.SingleLine = true
	p.path.Submit = true

	if p.rows == nil {
		p.rows = map[string]*kit.Row{}
	}

	c, s := p.conn(a)

	if s == nil {
		return a.Kit().EmptyState(gtx, icons.Folder, "No SSH session",
			"Open an SSH connection to browse and transfer files on the server.")
	}

	if c == nil {
		return a.Kit().EmptyState(gtx, icons.Sync, "Waiting for connection…", "The panel activates once the session is connected.")
	}

	cwd, entries, loading, err := c.snapshot()

	if c.sftp() == nil {
		if err != nil {
			return a.Kit().EmptyState(gtx, icons.Error, "SFTP unavailable", err.Error())
		}

		return a.Kit().EmptyState(gtx, icons.Sync, "Opening SFTP…", "")
	}

	if cwd != p.lastCwd {
		p.lastCwd = cwd
		p.path.SetText(cwd)
		p.selected = ""
	}

	p.handle(a, gtx, c, cwd)
	p.keys(a, gtx, c, cwd, entries)

	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 8, Right: 8, Bottom: 6}.Layout(gtx, func(gtx C) D {
				btn := func(clk *widget.Clickable, ic *icons.Icon) layout.FlexChild {
					return layout.Rigid(func(gtx C) D {
						return k.IconButtonSized(gtx, clk, ic, false, 26, 18, th.TextMuted)
					})
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					btn(&p.up, icons.ArrowUp),
					btn(&p.reload, icons.Refresh),
					btn(&p.mkdir, icons.NewFolder),
					btn(&p.upload, icons.Upload),
					btn(&p.download, icons.Download),
					layout.Rigid(func(gtx C) D {
						if _, cwd := s.Terminal().Cwd(); cwd == "" {
							return D{}
						}

						return k.IconButtonSized(gtx, &p.follow, icons.Terminal, false, 26, 18, th.TextMuted)
					}),
					layout.Flexed(1, func(gtx C) D { return D{} }),
					layout.Rigid(func(gtx C) D {
						if loading {
							return k.Small(gtx, "Loading…")
						}

						return k.Small(gtx, fmt.Sprintf("%d items", len(entries)))
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 10, Right: 10, Bottom: 6}.Layout(gtx, func(gtx C) D {
				return k.Field(gtx, &p.path, kit.FieldOpts{Mono: true, Icon: icons.FolderOpen})
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
			event.Op(gtx.Ops, p)
			area.Pop()

			if len(entries) == 0 && !loading {
				return k.EmptyState(gtx, icons.FolderOpen, "This folder is empty", "Drag files onto the window or use Upload to add some.")
			}

			return k.List(gtx, &p.list, len(entries), func(gtx C, i int) D {
				return p.entryRow(a, gtx, c, cwd, entries[i])
			})
		}),
		layout.Rigid(func(gtx C) D { return p.transfers(a, gtx) }),
	)

	k.Menu(gtx, &p.menu)
	return dims
}

func (p *Panel) handle(a core.Host, gtx C, c *sftpConn, cwd string) {
	if p.follow.Clicked(gtx) {
		if _, dir := a.ActiveSession().Terminal().Cwd(); dir != "" {
			p.load(a, c, dir)
		}
	}

	if p.up.Clicked(gtx) {
		p.load(a, c, path.Dir(cwd))
	}

	if p.reload.Clicked(gtx) {
		p.load(a, c, cwd)
	}

	if p.upload.Clicked(gtx) {
		go p.uploadFile(a, c, cwd)
	}

	if p.download.Clicked(gtx) && p.selected != "" {
		p.downloadEntry(a, c, cwd, p.selected)
	}

	if p.mkdir.Clicked(gtx) {
		f := dialogs.NewForm(a.Kit(), "New folder", "Create", dialogs.NewField("Name", "folder", ""))
		f.OnOK = func(v []string) bool {
			name := strings.TrimSpace(v[0])

			if name == "" {
				return false
			}

			go func() {
				if err := c.sftp().Mkdir(path.Join(cwd, name)); err != nil {
					a.Toasts().Error("Couldn't create the folder", err.Error())
					return
				}

				p.load(a, c, cwd)
			}()
			return true
		}
		a.OpenIn(p, f)
	}

	for {
		ev, ok := p.path.Update(gtx)

		if !ok {
			break
		}

		if sub, ok := ev.(widget.SubmitEvent); ok {
			p.load(a, c, strings.TrimSpace(sub.Text))
		}
	}

	if i, ok := p.menu.Chosen(); ok {
		e := p.pick
		full := path.Join(cwd, e.Name)

		switch i {
		case sftpMenuOpen:
			if e.IsDir {
				p.load(a, c, full)
			} else {
				go p.downloadFile(a, c, full)
			}
		case sftpMenuEdit:
			p.edit(a, c, full, cwd)
		case sftpMenuDownload:
			p.downloadEntry(a, c, cwd, e.Name)
		case sftpMenuCopyPath:
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(full))})
		case sftpMenuRename:
			p.rename(a, c, cwd, e)
		case sftpMenuDelete:
			p.remove(a, c, cwd, e)
		}
	}
}

func (p *Panel) entryRow(a core.Host, gtx C, c *sftpConn, cwd string, e sshx.Entry) D {
	k := a.Kit()
	th := a.Kit().Th

	r := p.rows[e.Name]

	if r == nil {
		r = &kit.Row{}
		p.rows[e.Name] = r
	}

	ev := r.Update(gtx)

	if ev.Click {
		p.selected = e.Name
		gtx.Execute(key.FocusCmd{Tag: p})
	}

	if ev.DoubleClick {
		if e.IsDir {
			p.load(a, c, path.Join(cwd, e.Name))
		} else {
			go p.downloadFile(a, c, path.Join(cwd, e.Name))
		}
	}

	if ev.Context {
		p.selected = e.Name
		p.pick = e
		open := "Download"

		if e.IsDir {
			open = "Open"
		}

		p.menu.Open([]kit.MenuItem{
			sftpMenuOpen:     {Label: open, Icon: icons.FolderOpen},
			sftpMenuEdit:     {Label: "Edit", Icon: icons.Edit, Disabled: e.IsDir},
			sftpMenuDownload: {Label: "Download…", Icon: icons.Download},
			sftpMenuCopyPath: {Label: "Copy path", Icon: icons.Copy},
			sftpMenuRename:   {Label: "Rename", Icon: icons.Edit, Shortcut: "F2"},
			sftpMenuSep:      kit.Separator(),
			sftpMenuDelete:   {Label: "Delete", Icon: icons.Delete, Danger: true, Shortcut: "Del"},
		}, ev.At.Add(image.Pt(-gtx.Dp(120), gtx.Dp(4))))
	}

	ic, col := entryStyle(th, e)

	dims := k.Row(gtx, r, kit.RowOpts{Selected: p.selected == e.Name, Focused: true, Height: 46}, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 16, col) }),
			layout.Rigid(kit.Gap(10)),
			layout.Flexed(1, func(gtx C) D {
				w := font.Normal

				if e.IsDir {
					w = font.Medium
				}

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D { return k.Label(gtx, kit.TextStyle{Weight: w, MaxLines: 1}, e.Name) }),
							layout.Rigid(func(gtx C) D {
								if e.IsDir {
									return D{}
								}

								return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, format.Size(e.Size))
							}),
						)
					}),
					layout.Rigid(kit.Gap(2)),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D {
								return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: th.TextFaint}, format.FileTime(e.ModTime))
							}),
							layout.Rigid(func(gtx C) D {
								return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: th.TextFaint, Mono: true}, e.Mode.Perm().String())
							}),
						)
					}),
				)
			}),
		)
	})

	p.trackDrag(a, gtx, c, cwd, e, dims.Size)
	return dims
}

func New(h core.Host) *Panel {
	return &Panel{h: h}
}

type (
	C = core.C
	D = core.D
)

func (p *Panel) Dir(sessID string) string {
	c := p.connFor(sessID)

	if c == nil {
		return ""
	}

	cwd, _, _, _ := c.snapshot()
	return cwd
}
