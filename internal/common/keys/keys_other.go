//go:build !darwin || ios

package keys

import (
	"strings"

	"gioui.org/io/key"
)

const (
	Command key.Modifiers = 0
	linkMod               = key.ModCtrl
)

var appMods = []key.Modifiers{key.ModCtrl | key.ModShift}

var labels *strings.Replacer
