package app

import (
	gioapp "gioui.org/app"

	"github.com/northlighthq/boreal/internal/platform/titlebar"
)

const trafficLightsX = 20

func (a *App) attachView(ev gioapp.ViewEvent) {
	if e, ok := ev.(gioapp.AppKitViewEvent); ok {
		a.view = e.View
		a.configured()
	}
}

func (a *App) configured() {
	titlebar.Attach(a.view, trafficLightsX, float32(headerHeight-tabHeight/2), float32(headerHeight))
}
