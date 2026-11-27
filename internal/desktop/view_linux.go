//go:build linux && !android

package desktop

import (
	gioapp "gioui.org/app"

	"github.com/northlighthq/boreal/internal/platform/dragdrop"
)

func (a *App) attachView(ev gioapp.ViewEvent) {
	switch e := ev.(type) {
	case gioapp.WaylandViewEvent:
		dragdrop.EnableWayland(e.Display, e.Surface, a.dropFiles, a.dragHover)
	case gioapp.X11ViewEvent:
		dragdrop.EnableX11(e.Window, a.dropFiles, a.dragHover)
	}
}

func (a *App) configured() {}
