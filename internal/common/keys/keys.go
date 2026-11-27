package keys

import (
	"gioui.org/io/key"
)

var App = appMods

const Link = linkMod

func IsApp(m key.Modifiers) bool {
	for _, a := range App {
		if m == a {
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
