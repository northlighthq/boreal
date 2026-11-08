package mobile

import (
	gioapp "gioui.org/app"
	"gioui.org/io/event"

	"github.com/northlighthq/boreal/internal/platform/android"
)

func (a *App) platformEvent(e event.Event) {
	if v, ok := e.(gioapp.AndroidViewEvent); ok {
		android.RegisterBack(v.View)
	}
}
