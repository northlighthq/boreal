package snippets

import (
	"errors"
	"strconv"
	"strings"

	"gioui.org/io/key"
)

var (
	errNoKey      = errors.New("Add a key, like Ctrl+Alt+F or F23.")
	errBadKey     = errors.New("Use a letter, a digit or a function key from F1 to F24.")
	errNeedMod    = errors.New("Letters and digits need Alt or Cmd, so they don't steal terminal keys.")
	errReserved   = errors.New("Boreal or the terminal already uses this shortcut.")
	errBadTrigger = errors.New("A trigger needs at least 2 characters and no spaces, like ;;firewall.")
)

type Binding struct {
	Name key.Name
	Mods key.Modifiers
}

var modNames = []struct {
	mod  key.Modifiers
	name string
}{
	{key.ModCtrl, "Ctrl"},
	{key.ModAlt, "Alt"},
	{key.ModShift, "Shift"},
	{key.ModCommand, "Cmd"},
	{key.ModSuper, "Super"},
}

var modAliases = map[string]key.Modifiers{
	"ctrl": key.ModCtrl, "control": key.ModCtrl,
	"alt": key.ModAlt, "option": key.ModAlt, "opt": key.ModAlt,
	"shift": key.ModShift,
	"cmd": key.ModCommand, "command": key.ModCommand,
	"super": key.ModSuper, "win": key.ModSuper, "meta": key.ModSuper,
}

func ParseShortcut(s string) (Binding, error) {
	var b Binding

	for _, part := range strings.Split(s, "+") {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		if m, ok := modAliases[strings.ToLower(part)]; ok {
			b.Mods |= m
			continue
		}

		if b.Name != "" {
			return Binding{}, errBadKey
		}

		name, ok := keyName(part)

		if !ok {
			return Binding{}, errBadKey
		}

		b.Name = name
	}

	if b.Name == "" {
		return Binding{}, errNoKey
	}

	if !b.function() && !b.Mods.Contain(key.ModAlt) && !b.Mods.Contain(key.ModCommand) {
		return Binding{}, errNeedMod
	}

	if b.reserved() {
		return Binding{}, errReserved
	}

	return b, nil
}

func keyName(s string) (key.Name, bool) {
	up := strings.ToUpper(s)

	if len(up) == 1 && (up[0] >= 'A' && up[0] <= 'Z' || up[0] >= '0' && up[0] <= '9') {
		return key.Name(up), true
	}

	if n, err := strconv.Atoi(strings.TrimPrefix(up, "F")); err == nil && strings.HasPrefix(up, "F") && n >= 1 && n <= 24 {
		return key.Name("F" + strconv.Itoa(n)), true
	}

	return "", false
}

func (b Binding) function() bool {
	return len(b.Name) > 1 && b.Name[0] == 'F'
}

func (b Binding) reserved() bool {
	ctrlShift := key.ModCtrl | key.ModShift

	switch {
	case b.Mods == ctrlShift:
		return true
	case b.Mods == key.ModAlt && len(b.Name) == 1 && b.Name[0] >= '1' && b.Name[0] <= '6':
		return true
	case b.Mods == key.ModCtrl|key.ModAlt && b.Name == "S":
		return true
	}

	return false
}

func (b Binding) String() string {
	var parts []string

	for _, m := range modNames {
		if b.Mods.Contain(m.mod) {
			parts = append(parts, m.name)
		}
	}

	return strings.Join(append(parts, string(b.Name)), "+")
}

func (b Binding) Filter() key.Filter {
	return key.Filter{Name: b.Name, Required: b.Mods}
}

func ValidTrigger(t string) bool {
	return len([]rune(t)) >= 2 && !strings.ContainsAny(t, " \t\r\n")
}
