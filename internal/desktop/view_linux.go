//go:build linux && !android

package desktop

import (
	gioapp "gioui.org/app"

	"github.com/northlighthq/boreal/internal/platform/dragdrop"
)

func (a *App) attachView(ev gioapp.ViewEvent) {
	if e, ok := ev.(gioapp.WaylandViewEvent); ok {
		dragdrop.EnableWayland(e.Display, e.Surface, a.dropFiles, a.dragHover)
	}
}

func (a *App) configured() {}
