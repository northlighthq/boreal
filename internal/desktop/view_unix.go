//go:build !windows && !darwin

package desktop

import gioapp "gioui.org/app"

func (a *App) attachView(ev gioapp.ViewEvent) {}

func (a *App) configured() {}
