package keys

import (
	"gioui.org/io/key"
)

var App = appMods

const Link = linkMod

func IsApp(modifiers key.Modifiers) bool {
	for _, mods := range App {
		if modifiers == mods {
			return true
		}
	}

	return false
}

func Label(s string) string {
	if labels == nil {
		return s
	}

	return labels.Replace(s)
}
