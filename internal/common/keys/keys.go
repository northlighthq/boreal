// Package keys holds the per-platform side of Boreal's keyboard shortcuts.
package keys

import (
	"gioui.org/io/key"
)

// App lists the modifier sets that trigger Boreal's own shortcuts (Ctrl+Shift+T and so on).
// macOS also takes Command, the way its own apps do.
var App = appMods

// Link is the modifier that turns a terminal URL into a clickable link.
const Link = linkMod

// IsApp reports whether m is one of the App modifier sets.
func IsApp(m key.Modifiers) bool {
	for _, a := range App {
		if m == a {
			return true
		}
	}

	return false
}

// Label rewrites a shortcut written the Windows and Linux way ("Ctrl+Shift+T")
// into how the current platform shows it.
func Label(s string) string {
	if labels == nil {
		return s
	}

	return labels.Replace(s)
}
