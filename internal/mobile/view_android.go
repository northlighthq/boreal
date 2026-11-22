package mobile

import (
	gioapp "gioui.org/app"
	"gioui.org/io/event"

	"github.com/northlighthq/boreal/internal/platform/android"
)

func (a *App) platformEvent(e event.Event) {
	if v, ok := e.(gioapp.AndroidViewEvent); ok {
		a.view = v.View
		android.RegisterBack(v.View)
	}
}

func (a *App) keepSessions(n int) {
	if n == 0 {
		android.ReleaseSessions()
		return
	}

	if !a.asked {
		a.asked = true
		android.AskNotifications(a.view)
	}

	android.KeepSessions("Boreal", sessionsText(n))
}
