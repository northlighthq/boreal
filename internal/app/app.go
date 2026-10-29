package app

import (
	"fmt"
	"image"
	"net/url"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	gioapp "gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/x/explorer"
	"github.com/northlighthq/glyphs"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/app/hosts"
	"github.com/northlighthq/boreal/internal/app/onboarding"
	"github.com/northlighthq/boreal/internal/app/palette"
	"github.com/northlighthq/boreal/internal/app/settings"
	"github.com/northlighthq/boreal/internal/app/snippets"
	"github.com/northlighthq/boreal/internal/app/welcome"
	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/fonts"
	"github.com/northlighthq/boreal/internal/platform/shell"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/transfer"
	"github.com/northlighthq/boreal/internal/tunnel"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
	"github.com/northlighthq/boreal/internal/ui/toast"
	"github.com/northlighthq/boreal/internal/vault"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	appName      = "Boreal"
	windowWidth  = 1280
	windowHeight = 800
	minWidth     = 640
	minHeight    = 420
)

type App struct {
	win      *gioapp.Window
	view     uintptr
	explorer *explorer.Explorer
	kit      *kit.Kit
	shaper   *text.Shaper
	face     font.Font
	glyphs   *glyphs.Set

	settings config.Settings
	vault    *vault.Vault
	profiles []shell.Profile

	ws        workspace
	tools     tools
	chrome    chrome
	welcome   *welcome.Screen
	palette   *palette.Palette
	toasts    toast.Stack
	tunnels   *tunnel.Manager
	transfers *transfer.Queue

	modal       core.Modal
	leaving     core.Modal
	leaveAt     time.Time
	docked      [2]core.Modal
	prompts     promptQueue
	passes      passphrases
	lock        lockScreen
	started     time.Time
	restored    bool
	drops       chan []string
	dragOver    atomic.Bool
	extKeys     chan string
	snipKeys    snippetKeys
	clips       chan serverClip
	allowedClip *serverClip

	maximized  bool
	fullscreen bool
	closing    bool
	hideAddrs  atomic.Bool
	autoUpdate atomic.Bool
	updates    updater
	keepAlive  atomic.Int64
	vaultRev   uint64
	mu         sync.Mutex
}

func Run() error {
	v, err := vault.Open()

	if err != nil {
		return err
	}

	a := &App{
		win:      new(gioapp.Window),
		settings: config.Load(),
		vault:    v,
		profiles: shell.Detect(),
		drops:    make(chan []string, 8),
		extKeys:  make(chan string, 8),
		clips:    make(chan serverClip, 4),
	}
	a.syncLive()
	a.shaper, a.face = fonts.Load()
	a.glyphs, _ = fonts.Glyphs(a.settings.TerminalFont)
	a.kit = kit.New(a.buildTheme(), a.shaper)
	a.explorer = explorer.NewExplorer(a.win)
	a.tunnels = a.newTunnels()
	a.transfers = transfer.New(a.Invalidate)
	a.tools.init(a)
	a.started = time.Now()
	a.welcome = welcome.New(a)
	a.palette = palette.New(a.kit, a.paletteEntries, a.paletteQuick, a.ws.focusActive)

	if !a.settings.Onboarded {
		a.Open(onboarding.New(a))
	}

	a.toasts.OnChange(a.Invalidate)

	a.win.Option(
		gioapp.Title(appName),
		gioapp.Size(unit.Dp(windowWidth), unit.Dp(windowHeight)),
		gioapp.MinSize(unit.Dp(minWidth), unit.Dp(minHeight)),
		gioapp.Decorated(false),
	)

	a.startUpdates()
	return a.loop()
}

func (a *App) loop() error {
	var ops op.Ops

	for {
		ev := a.win.Event()
		a.explorer.ListenEvents(ev)

		switch e := ev.(type) {
		case gioapp.DestroyEvent:
			a.shutdown()
			a.applyUpdate()
			return e.Err
		case *gioapp.ClosingEvent:
			if a.confirmClose() {
				e.Abort()
			}
		case gioapp.ConfigEvent:
			a.maximized = e.Config.Mode == gioapp.Maximized
			a.fullscreen = e.Config.Mode == gioapp.Fullscreen
			a.configured()
		case gioapp.ViewEvent:
			a.attachView(e)
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, e)
			a.frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) confirmClose() bool {
	if a.closing || !a.settings.ConfirmClose {
		return false
	}

	ssh := 0

	for _, t := range a.ws.tabs {
		for _, p := range t.root.panes(nil) {
			if st, _ := p.sess.State(); p.sess.Kind == session.SSH && st == session.Connected {
				ssh++
			}
		}
	}

	tunnels := a.tunnels.Active()

	if ssh == 0 && tunnels == 0 {
		return false
	}

	msg := fmt.Sprintf("There are %d SSH sessions and %d active tunnels. All of them will be closed.", ssh, tunnels)
	a.Open(dialogs.NewConfirm(a.kit, "Quit Boreal?", msg, "Quit", func() {
		a.closing = true
		a.win.Perform(system.ActionClose)
	}))
	a.Invalidate()
	return true
}

func (a *App) shutdown() {
	a.saveTabs()
	a.ws.closeAll()
	a.tunnels.StopAll()
	config.Save(a.settings)
}

func (a *App) Invalidate() {
	a.win.Invalidate()
}

func (a *App) th() *theme.Theme {
	return a.kit.Th
}

var Frames atomic.Int64

func (a *App) frame(gtx C) {
	Frames.Add(1)

	if a.vault.Locked() {
		a.ws.reap(a)
		gtx.Constraints.Min = gtx.Constraints.Max
		a.kit.ReadPointer(gtx)
		a.lockLayout(gtx)
		a.toasts.Layout(gtx, a.kit)
		a.kit.TrackPointer(gtx)
		return
	}

	select {
	case paths := <-a.drops:
		a.handleDrop(paths)
	case name := <-a.extKeys:
		if a.modal == nil {
			a.runSnippetKey(name)
		}
	default:
	}

	a.handleShortcuts(gtx)
	a.ws.reap(a)
	a.applyServerCopies(gtx)
	a.restoreTabs()

	if rev := a.vault.Revision(); rev != a.vaultRev {
		a.vaultRev = rev
		a.tools.refresh(a)
	}

	gtx.Constraints.Min = gtx.Constraints.Max
	a.kit.ReadPointer(gtx)
	a.kit.Redact = a.redactor()
	if _, intro := a.modal.(*onboarding.Intro); !intro {
		a.chrome.shell(a, gtx)
		a.dropOverlay(gtx)
	}

	a.toasts.Layout(gtx, a.kit)

	if a.palette.IsOpen() {
		a.palette.Layout(gtx)
	}

	a.layoutModal(gtx)
	a.idleLock(gtx)
	a.kit.TrackPointer(gtx)

	if a.vault.Revision() != a.vaultRev {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (a *App) layoutModal(gtx C) {
	if a.modal == nil {
		if p := a.prompts.peek(); p != nil {
			a.modal = p
		}
	}

	if m := a.leaving; m != nil {
		if p := float32(gtx.Now.Sub(a.leaveAt)) / float32(160*time.Millisecond); p < 1 && m != a.modal {
			a.kit.Exit = max(1-p, 0.001)
			m.Layout(gtx.Disabled())
			a.kit.Exit = 0
			gtx.Execute(op.InvalidateCmd{})
		} else {
			a.leaving = nil
		}
	}

	if a.modal == nil {
		return
	}

	if a.modal.Layout(gtx) {
		a.leaving, a.leaveAt = a.modal, gtx.Now
		a.modal = nil
		a.ws.focusActive()
	}
}

func (a *App) Open(m core.Modal) {
	a.palette.Close()
	a.modal = m
}

var shortcuts = shortcutFilters()

func shortcutFilters() []event.Filter {
	cs := key.ModCtrl | key.ModShift
	return []event.Filter{
		key.Filter{Name: "P", Required: cs},
		key.Filter{Name: "T", Required: cs},
		key.Filter{Name: "W", Required: cs},
		key.Filter{Name: "D", Required: cs},
		key.Filter{Name: "E", Required: cs},
		key.Filter{Name: "N", Required: cs},
		key.Filter{Name: "L", Required: cs},
		key.Filter{Name: "B", Required: cs},
		key.Filter{Name: key.NameTab, Required: key.ModCtrl, Optional: key.ModShift},
		key.Filter{Name: key.NamePageDown, Required: key.ModCtrl},
		key.Filter{Name: key.NamePageUp, Required: key.ModCtrl},
		key.Filter{Name: "1", Required: key.ModAlt},
		key.Filter{Name: "2", Required: key.ModAlt},
		key.Filter{Name: "3", Required: key.ModAlt},
		key.Filter{Name: "4", Required: key.ModAlt},
		key.Filter{Name: "5", Required: key.ModAlt},
		key.Filter{Name: "6", Required: key.ModAlt},
		key.Filter{Name: key.NameLeftArrow, Required: key.ModAlt},
		key.Filter{Name: key.NameRightArrow, Required: key.ModAlt},
		key.Filter{Name: key.NameUpArrow, Required: key.ModAlt},
		key.Filter{Name: key.NameDownArrow, Required: key.ModAlt},
		key.Filter{Name: ",", Required: key.ModCtrl},
		key.Filter{Name: "S", Required: key.ModCtrl | key.ModAlt},
	}
}

func (a *App) handleShortcuts(gtx C) {
	filters := a.snippetKeys().filters

	for {
		ev, ok := gtx.Event(filters...)

		if !ok {
			return
		}

		e, ok := ev.(key.Event)

		if !ok {
			continue
		}

		if e.State != key.Press || a.modal != nil {
			continue
		}

		if a.runSnippetKey(snippets.Binding{Name: e.Name, Mods: e.Modifiers}.String()) {
			continue
		}

		a.shortcut(e)
	}
}

func (a *App) shortcut(e key.Event) {
	shift := e.Modifiers.Contain(key.ModShift)

	switch e.Name {
	case "P":
		a.palette.Show()
	case "T":
		a.NewLocalTab(a.DefaultProfile())
	case "W":
		a.ws.closeActivePane(a)
	case "D":
		a.ws.split(a, true)
	case "E":
		a.ws.split(a, false)
	case "N":
		a.OpenDocked(core.Right, hosts.NewEditor(a, vault.Host{}))
	case "L":
		a.LockVault()
	case "B":
		a.toggleActiveBroadcast()
	case key.NameTab:
		if shift {
			a.ws.cycle(-1)
		} else {
			a.ws.cycle(1)
		}
	case key.NamePageDown:
		a.ws.cycle(1)
	case key.NamePageUp:
		a.ws.cycle(-1)
	case "1", "2", "3", "4", "5", "6":
		a.tools.toggleIndex(a, int(e.Name[0]-'1'))
	case key.NameLeftArrow:
		a.ws.moveFocus(-1, 0)
	case key.NameRightArrow:
		a.ws.moveFocus(1, 0)
	case key.NameUpArrow:
		a.ws.moveFocus(0, -1)
	case key.NameDownArrow:
		a.ws.moveFocus(0, 1)
	case ",", "S":
		a.Open(settings.New(a))
	}
}

func (a *App) SetTheme(name string) {
	a.settings.Theme = name
	a.kit.SetTheme(a.buildTheme())
	a.ws.applyTheme(a)
	config.Save(a.settings)
}

func (a *App) buildTheme() *theme.Theme {
	s := a.settings
	return theme.Build(s.Theme, theme.Options{Accent: s.Accent, Density: s.Density, Corners: s.Corners, Terminal: s.TermScheme})
}

func (a *App) applySettings() {
	if g, err := fonts.Glyphs(a.settings.TerminalFont); err == nil {
		a.glyphs = g
	}

	a.ws.applySettings(a)
	config.Save(a.settings)
}

func openURL(raw string) bool {
	u, err := url.Parse(raw)

	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}

	return exec.Command("rundll32", "url.dll,FileProtocolHandler", u.String()).Start() == nil
}

func moveArea(gtx C, size image.Point) {
	defer clipRect(gtx, size).Pop()
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
}
