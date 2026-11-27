//go:build linux && !android

package desktop

import (
	gioapp "gioui.org/app"

	"github.com/northlighthq/boreal/internal/platform/dragdrop"
)

func (a *App) attachView(event gioapp.ViewEvent) {
	switch view := event.(type) {
	case gioapp.WaylandViewEvent:
		dragdrop.EnableWayland(view.Display, view.Surface, a.dropFiles, a.dragHover)
	case gioapp.X11ViewEvent:
		dragdrop.EnableX11(view.Window, a.dropFiles, a.dragHover)
	}
}

func (a *App) configured() {}
