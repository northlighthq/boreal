//go:build darwin && !ios

package keys

import (
	"strings"

	"gioui.org/io/key"
)

const (
	Command = key.ModCommand
	linkMod = key.ModCommand
)

var appMods = []key.Modifiers{key.ModCtrl | key.ModShift, key.ModCommand}

var labels = strings.NewReplacer(
	"Ctrl+Shift+Tab", "⌃⇧Tab",
	"Ctrl+Tab", "⌃Tab",
	"Ctrl+PgUp", "⌃PgUp",
	"Ctrl+PgDn", "⌃PgDn",
	"Ctrl+Alt+S", "⌘,",
	"Ctrl+click", "⌘-click",
	"Ctrl+Shift+", "⌘",
	"Ctrl+", "⌘",
	"Alt+", "⌥",
	"Shift+", "⇧",
)
