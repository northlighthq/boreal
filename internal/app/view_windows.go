package app

import (
	gioapp "gioui.org/app"

	"github.com/northlighthq/boreal/internal/platform/dragdrop"
	"github.com/northlighthq/boreal/internal/platform/extkeys"
)

func (a *App) attachView(ev gioapp.ViewEvent) {
	e, ok := ev.(gioapp.Win32ViewEvent)

	if !ok || e.HWND == 0 {
		return
	}

	hwnd := e.HWND
	go a.win.Run(func() {
		dragdrop.Enable(hwnd, a.dropFiles, a.dragHover)
		extkeys.Enable(hwnd, a.extKey)
	})
}

func (a *App) extKey(name string) {
	select {
	case a.extKeys <- name:
		a.Invalidate()
	default:
	}
}

func (a *App) configured() {}
