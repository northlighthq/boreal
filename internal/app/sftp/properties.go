package sftp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/format"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

const (
	dirSizeLimit = 200_000
	propsWidth   = 460
	propsLabel   = unit.Dp(96)
	permColumn   = unit.Dp(72)
)

var errNoConnection = errors.New("the SFTP connection is closed")

var (
	permClasses = []string{"Owner", "Group", "Others"}
	permRights  = []string{"Read", "Write", "Execute"}
)

type properties struct {
	a    core.Host
	p    *Panel
	c    *sftpConn
	cwd  string
	name string

	mu     sync.Mutex
	props  sshx.Props
	names  sshx.Names
	usage  sshx.DirUsage
	sizing bool
	loaded bool
	err    error

	cancel  context.CancelFunc
	perms   [9]widget.Bool
	octal   widget.Editor
	owner   widget.Editor
	group   widget.Editor
	inside  widget.Bool
	lastOct string
	filled  bool

	dlg    kit.Dialog
	ok     widget.Clickable
	close  widget.Clickable
	saving atomic.Bool
	done   atomic.Bool
}

func (p *Panel) properties(a core.Host, c *sftpConn, cwd string, e sshx.Entry) {
	ctx, cancel := context.WithCancel(context.Background())
	d := &properties{a: a, p: p, c: c, cwd: cwd, name: e.Name, cancel: cancel}
	d.octal.SingleLine, d.owner.SingleLine, d.group.SingleLine = true, true, true
	a.OpenIn(p, d)
	go d.load(ctx)
}

func (d *properties) Dialog() *kit.Dialog {
	return &d.dlg
}

func (d *properties) full() string {
	return path.Join(d.cwd, d.name)
}

func (d *properties) load(ctx context.Context) {
	s := d.c.sftp()

	if s == nil {
		d.set(func() { d.err, d.loaded = errNoConnection, true })
		return
	}

	pr, err := s.Props(d.full())
	names := s.Names()
	d.set(func() {
		d.props, d.names, d.err, d.loaded, d.sizing = pr, names, err, true, err == nil && pr.IsDir && !pr.IsLink
	})

	if err != nil || !pr.IsDir || pr.IsLink {
		return
	}

	u, err := s.DirSize(ctx, d.full(), dirSizeLimit, func(u sshx.DirUsage) { d.set(func() { d.usage = u }) })

	if ctx.Err() != nil {
		return
	}

	d.set(func() {
		d.usage, d.sizing = u, false

		if err != nil {
			d.err = err
		}
	})
}

func (d *properties) set(fn func()) {
	d.mu.Lock()
	fn()
	d.mu.Unlock()
	d.a.Invalidate()
}

func (d *properties) snapshot() (sshx.Props, sshx.Names, sshx.DirUsage, bool, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.props, d.names, d.usage, d.sizing, d.loaded, d.err
}

func (d *properties) fill(pr sshx.Props, names sshx.Names) {
	d.filled = true
	d.setPerms(pr.Mode)
	d.lastOct = sshx.FormatMode(pr.Mode)
	d.octal.SetText(d.lastOct)
	d.owner.SetText(names.User(pr.UID))
	d.group.SetText(names.Group(pr.GID))
}

func (d *properties) setPerms(m os.FileMode) {
	for i := range d.perms {
		d.perms[i].Value = m.Perm()&(1<<(8-i)) != 0
	}
}

func (d *properties) mode(base os.FileMode) os.FileMode {
	m := base &^ os.ModePerm

	for i := range d.perms {
		if d.perms[i].Value {
			m |= 1 << (8 - i)
		}
	}

	return m
}

func (d *properties) sync(gtx C, pr sshx.Props) {
	changed := false

	for i := range d.perms {
		if d.perms[i].Update(gtx) {
			changed = true
		}
	}

	base := pr.Mode & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)

	if m, ok := sshx.ParseMode(d.octal.Text()); ok {
		base = m &^ os.ModePerm
	}

	if changed {
		d.lastOct = sshx.FormatMode(d.mode(base))
		d.octal.SetText(d.lastOct)
		return
	}

	if t := d.octal.Text(); t != d.lastOct {
		d.lastOct = t

		if m, ok := sshx.ParseMode(t); ok {
			d.setPerms(m)
		}
	}
}

func (d *properties) Layout(gtx C) bool {
	pr, names, usage, sizing, loaded, err := d.snapshot()

	if loaded && err == nil && !d.filled {
		d.fill(pr, names)
	}

	if d.filled {
		d.sync(gtx, pr)
	}

	if d.ok.Clicked(gtx) && d.filled && !d.saving.Load() {
		d.apply(pr, names)
	}

	closed := d.close.Clicked(gtx) || d.done.Load()
	k := d.a.Kit()
	th := k.Th

	closed = k.Dialog(gtx, &d.dlg, kit.DialogOpts{
		Title:    d.name,
		Subtitle: d.full(),
		Width:    propsWidth,
		Body: func(gtx C) D {
			if !loaded {
				return k.Label(gtx, kit.TextStyle{Color: th.TextMuted}, "Reading properties…")
			}

			if !d.filled {
				return k.Label(gtx, kit.TextStyle{Color: th.Danger, MaxLines: 4}, err.Error())
			}

			return d.body(gtx, pr, usage, sizing, err)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &d.close, kit.Ghost, nil, "Close") },
			func(gtx C) D {
				label := "Apply"

				if d.saving.Load() {
					label = "Applying…"
				}

				return k.Button(gtx, &d.ok, kit.Primary, nil, label)
			},
		),
	}) || closed

	if closed {
		d.cancel()
	}

	return closed
}

func (d *properties) body(gtx C, pr sshx.Props, usage sshx.DirUsage, sizing bool, err error) D {
	k := d.a.Kit()
	th := k.Th

	rows := []layout.FlexChild{
		d.info(gtx, "Type", kind(pr)),
	}

	if pr.IsLink {
		rows = append(rows, d.info(gtx, "Points to", pr.Target))
	}

	rows = append(rows,
		d.info(gtx, "Size", sizeText(pr, usage, sizing)),
		d.info(gtx, "Modified", stamp(pr.ModTime)),
	)

	if !pr.Accessed.IsZero() {
		rows = append(rows, d.info(gtx, "Accessed", stamp(pr.Accessed)))
	}

	if err != nil {
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger, MaxLines: 3}, err.Error())
		}))
	}

	rows = append(rows,
		layout.Rigid(kit.Gap(14)),
		layout.Rigid(func(gtx C) D { return d.grid(gtx) }),
		layout.Rigid(kit.Gap(12)),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return k.LabeledField(gtx, "Octal", &d.octal, kit.FieldOpts{Hint: "755", Mono: true})
				}),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D { return k.LabeledField(gtx, "Owner", &d.owner, kit.FieldOpts{Hint: "root"}) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D { return k.LabeledField(gtx, "Group", &d.group, kit.FieldOpts{Hint: "root"}) }),
			)
		}),
	)

	if pr.IsDir && !pr.IsLink {
		rows = append(rows,
			layout.Rigid(kit.Gap(12)),
			layout.Rigid(func(gtx C) D { return k.Checkbox(gtx, &d.inside, "Apply to everything inside") }),
			layout.Rigid(kit.Gap(4)),
			layout.Rigid(func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, "Folders get these permissions. Files get them without execute, unless they were already executable.")
			}),
		)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (d *properties) info(gtx C, label, value string) layout.FlexChild {
	k := d.a.Kit()
	th := k.Th

	return layout.Rigid(func(gtx C) D {
		return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Dp(propsLabel)
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, label)
				}),
				layout.Flexed(1, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, MaxLines: 2}, value)
				}),
			)
		})
	})
}

func (d *properties) grid(gtx C) D {
	k := d.a.Kit()
	th := k.Th
	cell := func(w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(permColumn)
			gtx.Constraints.Max.X = gtx.Dp(permColumn)
			return w(gtx)
		})
	}
	muted := kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}

	head := []layout.FlexChild{cell(func(gtx C) D { return D{} })}

	for _, r := range permRights {
		head = append(head, cell(func(gtx C) D { return k.Label(gtx, muted, r) }))
	}

	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, head...) }),
	}

	for ci, class := range permClasses {
		line := []layout.FlexChild{cell(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall}, class) })}

		for ri := range permRights {
			b := &d.perms[ci*3+ri]
			line = append(line, cell(func(gtx C) D { return k.Checkbox(gtx, b, "") }))
		}

		rows = append(rows, layout.Rigid(kit.Gap(8)), layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (d *properties) apply(pr sshx.Props, names sshx.Names) {
	a := d.a
	mode, ok := sshx.ParseMode(d.octal.Text())

	if !ok {
		a.Toasts().Error("Invalid permissions", "Use three or four octal digits, like 755 or 2775.")
		return
	}

	uid, okU := names.UserID(d.owner.Text())
	gid, okG := names.GroupID(d.group.Text())

	if !okU || !okG {
		a.Toasts().Error("Unknown owner or group", "Use a name from the server or a numeric ID.")
		return
	}

	same := sshx.FormatMode(mode) == sshx.FormatMode(pr.Mode)
	chown := uid != pr.UID || gid != pr.GID
	inside := d.inside.Value && pr.IsDir && !pr.IsLink

	if same && !chown && !inside {
		d.done.Store(true)
		return
	}

	d.saving.Store(true)

	go func() {
		err := d.save(mode, uid, gid, chown, inside)
		d.saving.Store(false)

		if err != nil {
			a.Toasts().Error("Couldn't change "+d.name, err.Error())
			a.Invalidate()
			return
		}

		a.Toasts().Success("Properties saved", d.name)
		d.p.load(a, d.c, d.cwd)
		d.done.Store(true)
		a.Invalidate()
	}()
}

func (d *properties) save(mode os.FileMode, uid, gid uint32, chown, inside bool) error {
	s := d.c.sftp()

	if s == nil {
		return errNoConnection
	}

	if err := s.ChmodTree(d.full(), mode, inside); err != nil {
		return err
	}

	if !chown {
		return nil
	}

	return s.ChownTree(d.full(), uid, gid, inside)
}

func kind(pr sshx.Props) string {
	switch {
	case pr.IsLink && pr.IsDir:
		return "Link to a folder"
	case pr.IsLink:
		return "Link"
	case pr.IsDir:
		return "Folder"
	case pr.Mode.IsRegular():
		return "File"
	}

	return strings.TrimPrefix(pr.Mode.Type().String(), "-")
}

func sizeText(pr sshx.Props, u sshx.DirUsage, sizing bool) string {
	if !pr.IsDir || pr.IsLink {
		return format.Size(pr.Size)
	}

	s := fmt.Sprintf("%s in %d files and %d folders", format.Size(u.Size), u.Files, u.Folders)

	switch {
	case sizing:
		return "Counting… " + s
	case !u.Complete:
		return "More than " + s
	}

	return s
}

func stamp(t time.Time) string {
	return t.Local().Format("2 Jan 2006, 15:04:05")
}
