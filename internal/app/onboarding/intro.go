package onboarding

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"gioui.org/f32"
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
	"github.com/northlighthq/boreal/internal/ui/sol"
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
	wideLayout  = 1050
)

var cursorStyles = []string{config.CursorBlock, config.CursorBar, config.CursorUnderline}

var says = [steps]string{
	stepWelcome:  "Hi! Let's get Boreal ready together, it only takes a minute.",
	stepLook:     "Let's make it yours! Try a theme, everything changes right away.",
	stepTerminal: "Which shell do you like? I'll open it every time you press Ctrl+Shift+T.",
	stepSecurity: "I keep your hosts and keys in an encrypted vault. Want to lock it with a password?",
	stepHosts:    "Now let's bring in your servers!",
	stepDone:     "Yay, we're done! These are the shortcuts I use all the time.",
}

var titles = [steps]string{
	stepWelcome:  "Welcome to Boreal",
	stepLook:     "Pick your look",
	stepTerminal: "Your terminal",
	stepSecurity: "Lock your vault",
	stepHosts:    "Your servers",
	stepDone:     "You're all set",
}

var sideY = [steps]float32{stepLook: 0.40, stepTerminal: 0.58, stepSecurity: 0.36, stepHosts: 0.56}

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

	sol     sol.Sol
	actor   actor
	born    time.Time
	stepAt  time.Time
	bursts  []burst
	blocker bool
	wide    bool
	area    image.Rectangle

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

func (d *Intro) sided(step int) bool {
	return d.wide && step != stepWelcome && step != stepDone
}

func (d *Intro) spot(step int, size image.Point, dp float32) spot {
	if d.sided(step) {
		return spot{center: f32.Pt(float32(size.X)*0.2, float32(size.Y)*sideY[step]), size: 124 * dp}
	}

	s := float32(140)

	if !d.wide {
		s = 92
	}

	return spot{center: f32.Pt(float32(size.X)/2, 64*dp+s*dp*0.75), size: s * dp}
}

func (d *Intro) goTo(step int, now time.Time) {
	if step == d.step {
		return
	}

	d.actor.jump(d.actor.current, now)
	d.step, d.stepAt, d.passErr = step, now, ""

	if step == stepDone {
		d.celebrate(now.Add(jumpTime), 26, 220)
	}
}

func (d *Intro) celebrate(at time.Time, count int, reach float32) {
	d.bursts = append(d.bursts, burst{born: at, count: count, reach: reach})
	d.sol.Cheer(at)
}

func (d *Intro) update(gtx C) bool {
	now := gtx.Now
	s := d.h.Settings()

	for i := range d.themes {
		if d.themes[i].Clicked(gtx) && theme.Names[i] != s.Theme {
			name := theme.Names[i]
			d.apply(func(s *config.Settings) { s.Theme = name })
			d.celebrate(now, 12, 120)
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
		d.celebrate(now, 12, 120)
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
	now := gtx.Now
	k := d.h.Kit()
	th := k.Th
	size := gtx.Constraints.Max
	dp := gtx.Metric.PxPerDp
	d.wide = size.X >= gtx.Dp(wideLayout)

	if d.born.IsZero() {
		d.born, d.stepAt, d.actor.landed = now, now, now
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

	stars(gtx, size, now.Sub(d.born).Seconds(), th.TextMuted, d.area)

	target := d.spot(d.step, size, dp)
	p := d.actor.pose(target, now, d.born, dp)
	d.actor.current = spot{center: p.center, size: p.size}

	d.content(gtx, target)
	d.drawBursts(gtx, target)

	d.sol.Follow = true
	d.sol.Backdrop = th.Window
	drawSol(gtx, k, &d.sol, p)

	if !d.actor.jumping(now) {
		since := now.Sub(d.actor.landed)
		alpha := float32(clamp01(since.Seconds() / 0.2))
		width := min(gtx.Dp(420), size.X-gtx.Dp(32))

		if d.sided(d.step) {
			width = min(width, int(float32(size.X)*0.34))
		}

		at := f32.Pt(p.center.X, target.center.Y+target.size/2+6*dp)
		bubble(gtx, k, says[d.step], typed(says[d.step], since-120*time.Millisecond), at, width, alpha)
	}

	d.h.TitleBar(gtx)
	gtx.Execute(op.InvalidateCmd{})
	return false
}

func (d *Intro) drawBursts(gtx C, target spot) {
	now := gtx.Now
	live := d.bursts[:0]

	for _, b := range d.bursts {
		if b.at == (f32.Point{}) {
			b.at = target.center
		}

		if now.Before(b.born) || b.draw(gtx, now, d.h.Kit().Th.Text) {
			live = append(live, b)
		}
	}

	d.bursts = live
}

func (d *Intro) content(gtx C, target spot) {
	k := d.h.Kit()
	th := k.Th
	size := gtx.Constraints.Max
	width := min(gtx.Dp(columnWidth), size.X-gtx.Dp(48))

	rec := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, size.Y)}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(g,
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeTitle, Weight: font.SemiBold}, titles[d.step])
		}),
		layout.Rigid(kit.Gap(22)),
		layout.Rigid(d.stepBody),
		layout.Rigid(kit.Gap(28)),
		layout.Rigid(d.footer),
	)
	call := rec.Stop()

	x := (size.X - width) / 2
	y := (size.Y - dims.Size.Y) / 2

	if d.sided(d.step) {
		x = max(int(float32(size.X)*0.4), x)
	} else {
		y = max(y, int(target.center.Y+target.size/2)+gtx.Dp(130))
	}

	e := float32(ease(clamp01(gtx.Now.Sub(d.stepAt).Seconds() / contentFade.Seconds())))
	y += int((1 - e) * float32(gtx.Dp(18)))

	d.area = image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x, y).Add(dims.Size)}

	if e < 1 {
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
