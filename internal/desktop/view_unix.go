//go:build !windows && !darwin && !(linux && !android)

package desktop

import gioapp "gioui.org/app"

func (a *App) attachView(ev gioapp.ViewEvent) {}

func (a *App) configured() {}
