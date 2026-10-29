package onboarding

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/hosts"
	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/sshconfig"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/mark"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

type (
	C = core.C
	D = core.D
)

const (
	stepWelcome = iota
	stepLook
	stepTerminal
	stepSecurity
	stepHosts
	stepDone
	steps
)

const (
	minPassword = 10
	columnWidth = 560
)

var cursorStyles = []string{config.CursorBlock, config.CursorBar, config.CursorUnderline}

var says = [steps]string{
	stepWelcome:  "Let's get Boreal ready. It only takes a minute.",
	stepLook:     "Pick a theme. Everything changes right away.",
	stepTerminal: "Choose the shell that opens with Ctrl+Shift+T.",
	stepSecurity: "Hosts and keys live in an encrypted vault. You can lock it with a password.",
	stepHosts:    "Bring in your servers.",
	stepDone:     "You're all set. These shortcuts are worth knowing.",
}

var titles = [steps]string{
	stepWelcome:  "Welcome to Boreal",
	stepLook:     "Pick your look",
	stepTerminal: "Your terminal",
	stepSecurity: "Lock your vault",
	stepHosts:    "Your servers",
	stepDone:     "You're all set",
}

var shortcuts = [][2]string{
	{"Search hosts, snippets and actions", "Ctrl+Shift+P"},
	{"New local terminal", "Ctrl+Shift+T"},
	{"New host", "Ctrl+Shift+N"},
	{"Split right / down", "Ctrl+Shift+D / E"},
	{"Tool panels", "Alt+1 … Alt+6"},
	{"Lock Boreal", "Ctrl+Shift+L"},
}

type Intro struct {
	h    core.Host
	step int

	stepAt  time.Time
	blocker bool

	themes   []widget.Clickable
	termFont kit.Select
	shell    kit.Select
	cursor   kit.Segmented

	password, confirm widget.Editor
	osUnlock          widget.Bool
	passErr           string

	sshHosts   int
	importSSH  widget.Clickable
	addHost    widget.Clickable
	imported   bool
	openEditor bool

	back, next, skip widget.Clickable
}

func New(h core.Host) *Intro {
	s := h.Settings()
	d := &Intro{h: h, themes: make([]widget.Clickable, len(theme.Names))}
	d.osUnlock.Value = true

	for _, e := range []*widget.Editor{&d.password, &d.confirm} {
		e.SingleLine, e.Mask = true, '•'
	}

	for i, f := range fonts.TerminalFonts {
		if f == s.TerminalFont {
			d.termFont.Value = i
		}
	}

	for i, p := range h.Profiles() {
		if p.ID == s.Shell {
			d.shell.Value = i
		}
	}

	for i, c := range cursorStyles {
		if c == s.CursorStyle {
			d.cursor.Value = i
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		entries, _ := sshconfig.Parse(filepath.Join(home, ".ssh", "config"))
		d.sshHosts = len(entries)
	}

	return d
}

func (d *Intro) apply(fn func(s *config.Settings)) {
	s := d.h.Settings()
	fn(&s)
	d.h.UpdateSettings(s)
}

func (d *Intro) finish() bool {
	d.apply(func(s *config.Settings) { s.Onboarded = true })

	if d.openEditor {
		d.h.OpenDocked(core.Right, hosts.NewEditor(d.h, hosts.Defaults()))
	}

	return true
}

func (d *Intro) savePassword() bool {
	pass := d.password.Text()

	if pass == "" {
		return true
	}

	switch {
	case len([]rune(pass)) < minPassword:
		d.passErr = fmt.Sprintf("Use at least %d characters, or leave it empty to skip.", minPassword)
		return false
	case pass != d.confirm.Text():
		d.passErr = "The passwords don't match."
		return false
	}

	v := d.h.Vault()

	if err := v.SetPassword("", pass); err != nil {
		d.passErr = err.Error()
		return false
	}

	if !d.osUnlock.Value {
		v.SetOSUnlock(false)
	}

	d.password.SetText("")
	d.confirm.SetText("")
	return true
}

func (d *Intro) goTo(step int, now time.Time) {
	if step == d.step {
		return
	}

	d.step, d.stepAt, d.passErr = step, now, ""
}

func (d *Intro) update(gtx C) bool {
	now := gtx.Now
	s := d.h.Settings()

	for i := range d.themes {
		if d.themes[i].Clicked(gtx) && theme.Names[i] != s.Theme {
			name := theme.Names[i]
			d.apply(func(s *config.Settings) { s.Theme = name })
		}
	}

	if f := fonts.TerminalFonts[d.termFont.Value]; f != s.TerminalFont {
		d.apply(func(s *config.Settings) { s.TerminalFont = f })
	}

	if ps := d.h.Profiles(); d.shell.Value < len(ps) && ps[d.shell.Value].ID != s.Shell {
		id := ps[d.shell.Value].ID
		d.apply(func(s *config.Settings) { s.Shell = id })
	}

	if c := cursorStyles[d.cursor.Value]; c != s.CursorStyle {
		d.apply(func(s *config.Settings) { s.CursorStyle = c })
	}

	if d.importSSH.Clicked(gtx) && !d.imported {
		d.imported = true
		d.h.ImportSSHConfig()
	}

	if d.addHost.Clicked(gtx) {
		d.openEditor = true
		d.goTo(stepDone, now)
	}

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})

		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			return d.finish()
		}
	}

	if d.skip.Clicked(gtx) {
		return d.finish()
	}

	if d.back.Clicked(gtx) && d.step > 0 {
		d.goTo(d.step-1, now)
	}

	if d.next.Clicked(gtx) {
		switch {
		case d.step == stepDone:
			return d.finish()
		case d.step == stepSecurity && !d.savePassword():
		default:
			d.goTo(d.step+1, now)
		}
	}

	return false
}

func (d *Intro) Layout(gtx C) bool {
	th := d.h.Kit().Th
	size := gtx.Constraints.Max

	if d.stepAt.IsZero() {
		d.stepAt = gtx.Now
	}

	if d.update(gtx) {
		return true
	}

	kit.Rect(gtx, image.Rectangle{Max: size}, th.Window)
	block := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &d.blocker)
	block.Pop()

	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &d.blocker, Kinds: pointer.Press | pointer.Release | pointer.Scroll}); !ok {
			break
		}
	}

	d.content(gtx)
	d.h.TitleBar(gtx)
	return false
}

func (d *Intro) content(gtx C) {
	k := d.h.Kit()
	th := k.Th
	size := gtx.Constraints.Max
	width := min(gtx.Dp(columnWidth), size.X-gtx.Dp(48))

	rec := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, size.Y)}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(g,
		layout.Rigid(func(gtx C) D { return mark.Paint(gtx, 30, kit.Fade(th.TextFaint, 0.55)) }),
		layout.Rigid(kit.Gap(28)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeTitle, Weight: font.SemiBold}, titles[d.step])
		}),
		layout.Rigid(kit.Gap(6)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 3}, says[d.step])
		}),
		layout.Rigid(kit.Gap(24)),
		layout.Rigid(d.stepBody),
		layout.Rigid(kit.Gap(28)),
		layout.Rigid(d.footer),
	)
	call := rec.Stop()

	x := (size.X - width) / 2
	y := max((size.Y-dims.Size.Y)/2, gtx.Dp(48))

	e := float32(ease(clamp01(gtx.Now.Sub(d.stepAt).Seconds() / contentFade.Seconds())))
	y += int((1 - e) * float32(gtx.Dp(12)))

	if e < 1 {
		gtx.Execute(op.InvalidateCmd{})
		defer paint.PushOpacity(gtx.Ops, max(e, 0.01)).Pop()
	}
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

func (d *Intro) stepBody(gtx C) D {
	switch d.step {
	case stepLook:
		return d.look(gtx)
	case stepTerminal:
		return d.terminal(gtx)
	case stepSecurity:
		return d.security(gtx)
	case stepHosts:
		return d.servers(gtx)
	case stepDone:
		return d.done(gtx)
	}

	return d.welcome(gtx)
}

func (d *Intro) footer(gtx C) D {
	k := d.h.Kit()
	next := "Next"

	switch d.step {
	case stepWelcome:
		next = "Let's go"
	case stepDone:
		next = "Start using Boreal"
	}

	children := []layout.FlexChild{
		layout.Rigid(d.dots),
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
	}

	if d.step < stepDone {
		children = append(children, layout.Rigid(func(gtx C) D { return k.Button(gtx, &d.skip, kit.Ghost, nil, "Skip intro") }), layout.Rigid(kit.Gap(8)))
	}

	if d.step > stepWelcome {
		children = append(children, layout.Rigid(func(gtx C) D { return k.Button(gtx, &d.back, kit.Secondary, nil, "Back") }), layout.Rigid(kit.Gap(8)))
	}

	children = append(children, layout.Rigid(func(gtx C) D { return k.Button(gtx, &d.next, kit.Primary, nil, next) }))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (d *Intro) dots(gtx C) D {
	th := d.h.Kit().Th
	children := make([]layout.FlexChild, 0, steps)

	for i := range steps {
		children = append(children, layout.Rigid(func(gtx C) D {
			c, w := th.TextFaint, gtx.Dp(6)

			if i == d.step {
				c, w = th.Text, gtx.Dp(18)
			}

			r := image.Rect(0, 0, w, gtx.Dp(6))
			kit.RRect(gtx, r, gtx.Dp(3), c)
			return layout.Inset{Right: 6}.Layout(gtx, func(gtx C) D { return D{Size: r.Max} })
		}))
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}
