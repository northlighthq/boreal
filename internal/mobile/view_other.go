//go:build !android

package mobile

import "gioui.org/io/event"

func (a *App) platformEvent(e event.Event) {}

func (a *App) keepSessions(n int) {}
