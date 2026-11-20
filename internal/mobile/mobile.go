package mobile

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	gioapp "gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/northlighthq/glyphs"
	"github.com/northlighthq/vt"

	"github.com/northlighthq/boreal/internal/common/fonts"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/termview"
	"github.com/northlighthq/boreal/internal/common/theme"
	"github.com/northlighthq/boreal/internal/common/toast"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/privacy"
	"github.com/northlighthq/boreal/internal/core/session"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/hosts"
	"github.com/northlighthq/boreal/internal/mobile/keys"
	"github.com/northlighthq/boreal/internal/mobile/lock"
	"github.com/northlighthq/boreal/internal/mobile/nav"
	"github.com/northlighthq/boreal/internal/mobile/settings"
	"github.com/northlighthq/boreal/internal/mobile/terminal"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	appName      = "Boreal"
	phoneWidth   = 412
	phoneHeight  = 915
	transition   = 220 * time.Millisecond
	slideDp      = 28
	maxJumpDepth = 4
	termCols     = 80
	termRows     = 24
	keepAlive    = 30 * time.Second
)

type App struct {
	win      *gioapp.Window
	kit      *kit.Kit
	glyphs   *glyphs.Set
	settings config.Settings
	vault    *vault.Vault
	toasts   toast.Stack
	data     vault.Data
	dataRev  uint64
	dataOK   bool

	stacks  [tabCount]*nav.Stack
	tab     tab
	tabBtns [tabCount]widget.Clickable
	back    widget.Clickable
	actions []ui.Action

	sheet    *ui.Sheet
	mu       sync.Mutex
	dialogs  []*ui.Dialog
	sessions []*terminal.Session
	lock     *lock.Screen
	setup    *lock.Setup
	locked   bool
	blur     bool
	now      time.Time
}

func Run() error {
	v, recovered, err := openVault()

	if err != nil {
		return err
	}

	a := &App{win: new(gioapp.Window), settings: config.Load(), vault: v}

	if recovered {
		a.toasts.Info("Your old vault was set aside", "It was protected by this phone's secure storage and can't be opened here. Boreal started a new one.")
	}
	shaper, _ := fonts.LoadEmbedded()
	a.kit = kit.New(a.buildTheme(), shaper)
	a.glyphs, _ = fonts.Glyphs(a.settings.TerminalFont)
	a.lock = lock.New(a)
	a.setup = lock.NewSetup(a)
	a.toasts.OnChange(a.Invalidate)
	a.stacks = [tabCount]*nav.Stack{
		tabHosts:    nav.NewStack(hosts.NewList(a)),
		tabSessions: nav.NewStack(terminal.NewList(a)),
		tabKeys:     nav.NewStack(keys.NewList(a)),
		tabSettings: nav.NewStack(settings.NewRoot(a)),
	}

	a.win.Option(gioapp.Title(appName), gioapp.Size(unit.Dp(phoneWidth), unit.Dp(phoneHeight)))
	a.applyBars()
	return a.loop()
}

func (a *App) loop() error {
	var ops op.Ops

	for {
		switch e := a.win.Event().(type) {
		case gioapp.DestroyEvent:
			for _, s := range a.sessions {
				s.Sess.Close()
			}

			return e.Err
		case gioapp.FrameEvent:
			gtx := a.context(&ops, e)
			a.frame(gtx)
			e.Frame(gtx.Ops)
		default:
			a.platformEvent(e)
		}
	}
}

func (a *App) context(ops *op.Ops, e gioapp.FrameEvent) C {
	ops.Reset()
	paint.Fill(ops, a.kit.Th.Panel)

	left, top := e.Metric.Dp(e.Insets.Left), e.Metric.Dp(e.Insets.Top)
	size := image.Pt(e.Size.X-left-e.Metric.Dp(e.Insets.Right), e.Size.Y-top-e.Metric.Dp(e.Insets.Bottom))
	op.Offset(image.Pt(left, top)).Add(ops)
	clip.Rect{Max: size}.Push(ops)

	return C{Ops: ops, Now: e.Now, Source: e.Source, Metric: e.Metric, Constraints: layout.Exact(size)}
}

func (a *App) frame(gtx C) {
	a.now = gtx.Now
	gtx.Constraints.Min = gtx.Constraints.Max
	a.kit.ReadPointer(gtx)
	a.kit.Redact = a.redactor()

	if a.vault.Locked() {
		a.forgetData()
		a.locked = true
		a.lock.Layout(gtx)
		a.toasts.Layout(gtx, a.kit)
		return
	}

	if a.vault.NeedsPassword() {
		a.locked = true
		a.setup.Layout(gtx)
		a.toasts.Layout(gtx, a.kit)
		return
	}

	if a.locked {
		a.locked, a.blur = false, true
	}

	if a.blur {
		a.blur = false
		gtx.Execute(key.FocusCmd{})
		gtx.Execute(key.SoftKeyboardCmd{Show: false})
	}

	a.handleBack(gtx)

	for i := range a.tabBtns {
		if a.tabBtns[i].Clicked(gtx) {
			a.selectTab(tab(i))
		}
	}

	if a.back.Clicked(gtx) {
		a.Pop()
	}

	a.prune()
	a.screen(gtx)
	a.overlays(gtx)
	a.kit.TrackPointer(gtx)
}

func (a *App) screen(gtx C) {
	s := a.stack()
	top := s.Top()
	_, full := top.(ui.FullScreen)
	a.actions = nil

	if ac, ok := top.(ui.Actionable); ok {
		a.actions = ac.Actions()
	}

	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if full {
				return D{}
			}

			return a.topBar(gtx)
		}),
		layout.Flexed(1, a.content),
		layout.Rigid(func(gtx C) D {
			if s.CanPop() {
				return D{}
			}

			return a.bottomBar(gtx)
		}),
	)
}

func (a *App) overlays(gtx C) {
	a.toasts.Layout(gtx, a.kit)

	if a.sheet != nil && a.sheet.Layout(gtx, a.kit) {
		a.sheet = nil
	}

	if d := a.dialog(); d != nil && d.Layout(gtx, a.kit) {
		a.mu.Lock()
		a.dialogs = slices.DeleteFunc(a.dialogs, func(x *ui.Dialog) bool { return x == d })
		a.mu.Unlock()
		a.Invalidate()
	}
}

func (a *App) content(gtx C) D {
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	p, back := a.stack().Transition(gtx.Now, transition)

	if p < 1 {
		gtx.Execute(op.InvalidateCmd{})
		e := 1 - (1-p)*(1-p)
		dir := float32(1)

		if back {
			dir = -1
		}

		dx := int(dir * (1 - e) * float32(gtx.Dp(slideDp)))
		defer op.Offset(image.Pt(dx, 0)).Push(gtx.Ops).Pop()
		defer paint.PushOpacity(gtx.Ops, e).Pop()
	}

	a.stack().Top().Layout(gtx)
	return D{Size: size}
}

func (a *App) handleBack(gtx C) {
	if a.sheet == nil && a.dialog() == nil && !a.stack().CanPop() && a.tab == tabHosts {
		return
	}

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameBack})

		if !ok {
			return
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			a.goBack()
		}
	}
}

func (a *App) goBack() {
	switch {
	case a.dialog() != nil:
		a.dialog().Dismiss()
	case a.sheet != nil:
		a.sheet.Dismiss()
	case a.stack().CanPop():
		a.Pop()
	default:
		a.selectTab(tabHosts)
	}
}

func (a *App) stack() *nav.Stack {
	return a.stacks[a.tab]
}

func (a *App) selectTab(t tab) {
	a.blur = true

	if t == a.tab {
		a.stack().PopToRoot(time.Now())
		return
	}

	a.tab = t
}

func (a *App) dialog() *ui.Dialog {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.dialogs) == 0 {
		return nil
	}

	return a.dialogs[0]
}

func (a *App) prune() {
	for _, s := range a.sessions {
		select {
		case <-s.Sess.Done():
			a.Close(s)
			return
		default:
		}
	}
}

func (a *App) Kit() *kit.Kit                 { return a.kit }
func (a *App) Vault() *vault.Vault           { return a.vault }
func (a *App) Settings() config.Settings     { return a.settings }
func (a *App) Invalidate()                   { a.win.Invalidate() }
func (a *App) Notify(title, message string)  { a.toasts.Info(title, message) }
func (a *App) Sessions() []*terminal.Session { return a.sessions }
func (a *App) ShowSheet(s *ui.Sheet)         { a.sheet = s; a.Invalidate() }

func (a *App) Ask(d *ui.Dialog) {
	a.mu.Lock()
	a.dialogs = append(a.dialogs, d)
	a.mu.Unlock()
	a.Invalidate()
}

func (a *App) UpdateSettings(s config.Settings) {
	old := a.settings
	a.settings = s
	config.Save(s)

	if s.Theme != old.Theme || s.Accent != old.Accent {
		a.kit.SetTheme(a.buildTheme())
		a.applyBars()
	}

	if s.TerminalFont != old.TerminalFont {
		if g, err := fonts.Glyphs(s.TerminalFont); err == nil {
			a.glyphs = g
		}
	}

	for _, t := range a.sessions {
		t.View.SetSettings(a.termSettings())
		t.View.SetTheme(a.kit.Th)
		t.View.SetFonts(a.glyphs)
	}

	a.Invalidate()
}

func (a *App) Connect(h vault.Host) {
	go a.vault.TouchHost(h.ID)

	s := session.New(session.Options{
		Kind:       session.SSH,
		Target:     a.target(h, 0),
		Vault:      a.vault,
		Hooks:      a.hooks(),
		Redact:     func(s string) string { return a.redact(s) },
		KeepAlive:  keepAlive,
		Cols:       termCols,
		Rows:       termRows,
		Scrollback: a.settings.Scrollback,
		Title:      h.Title(),
		OnChange:   a.Invalidate,
		OnConnect: func(c *sshx.Client) {
			go a.detectOS(h.ID, c)
		},
	})

	view := termview.New(s, a.kit, a.glyphs, a.termSettings())
	view.OnPaste = func(text string) bool { return a.guardPaste(view, h.Title(), text) }
	t := terminal.NewSession(s, view, h)
	a.sessions = append(a.sessions, t)
	a.Open(t)
}

func (a *App) Open(t *terminal.Session) {
	a.tab = tabSessions
	st := a.stack()
	st.PopToRoot(time.Now())
	st.Push(terminal.NewScreen(a, t), time.Now())
	a.Invalidate()
}

func (a *App) Close(t *terminal.Session) {
	i := slices.Index(a.sessions, t)

	if i < 0 {
		return
	}

	a.sessions = slices.Delete(a.sessions, i, i+1)
	t.Sess.Close()

	for _, st := range a.stacks {
		if sc, ok := st.Top().(*terminal.Screen); ok && sc.Session() == t {
			st.Pop(time.Now())
		}
	}

	a.Invalidate()
}

func (a *App) target(h vault.Host, depth int) sshx.Target {
	t := sshx.Target{Host: h}

	if h.Auth == vault.AuthKey && h.KeyID != "" {
		if k, ok := a.vault.Key(h.KeyID); ok {
			t.Key = &k
		}
	}

	if h.JumpID != "" && h.JumpID != h.ID && depth < maxJumpDepth {
		if j, ok := a.vault.Host(h.JumpID); ok {
			jt := a.target(j, depth+1)
			t.Jump = &jt
		}
	}

	return t
}

func (a *App) detectOS(id string, c *sshx.Client) {
	if id == "" {
		return
	}

	if name, err := sshx.DetectOS(c); err == nil && name != "" && a.vault.SetHostOS(id, name) == nil {
		a.Invalidate()
	}
}

func (a *App) termSettings() termview.Settings {
	s := a.settings
	style := vt.CursorBlock

	switch s.CursorStyle {
	case config.CursorBar:
		style = vt.CursorBar
	case config.CursorUnderline:
		style = vt.CursorUnderline
	}

	return termview.Settings{
		TextSize:    unit.Sp(s.FontSize),
		LineHeight:  s.LineHeight,
		CursorStyle: style,
		CursorBlink: s.CursorBlink,
		Ligatures:   s.Ligatures,
		Highlight:   s.Highlight,
		Touch:       true,
	}
}

func (a *App) redactor() func(string) string {
	if !a.settings.HideAddresses {
		return nil
	}

	return privacy.RedactIPs
}

func (a *App) redact(s string) string {
	if r := a.redactor(); r != nil {
		return r(s)
	}

	return s
}

func (a *App) buildTheme() *theme.Theme {
	s := a.settings
	return theme.Build(s.Theme, theme.Options{Accent: s.Accent, Density: s.Density, Corners: s.Corners, Terminal: s.TermScheme})
}

func (a *App) applyBars() {
	th := a.kit.Th
	a.win.Option(gioapp.StatusColor(th.Panel), gioapp.NavigationColor(th.Panel))
}

func (a *App) Push(s nav.Screen) {
	a.stack().Push(s, time.Now())
	a.blur = true
	a.Invalidate()
}

func (a *App) Pop() {
	a.stack().Pop(time.Now())
	a.blur = true
	a.Invalidate()
}

const androidOS = "android"

func (a *App) Data() vault.Data {
	if rev := a.vault.Revision(); !a.dataOK || rev != a.dataRev {
		a.data, a.dataRev, a.dataOK = a.vault.Snapshot(), rev, true
	}

	return a.data
}

func (a *App) forgetData() {
	a.data, a.dataOK = vault.Data{}, false
}

func openVault() (*vault.Vault, bool, error) {
	v, err := vault.Open()

	if !errors.Is(err, vault.ErrNeedUnlock) || runtime.GOOS != androidOS {
		return v, false, err
	}

	dir, derr := config.Dir()

	if derr != nil {
		return nil, false, err
	}

	path := filepath.Join(dir, vault.FileName)

	if rerr := os.Rename(path, path+".unreadable-"+time.Now().Format("20060102-150405")); rerr != nil {
		return nil, false, err
	}

	v, err = vault.Open()
	return v, true, err
}
