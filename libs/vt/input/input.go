package input

import (
	"fmt"
	"strings"

	"github.com/northlighthq/vt"
)

type Key uint8

const (
	KeyNone Key = iota
	KeyEnter
	KeyTab
	KeyBackspace
	KeyEscape
	KeyUp
	KeyDown
	KeyRight
	KeyLeft
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

type Mod uint8

const (
	ModShift Mod = 1 << iota
	ModAlt
	ModCtrl
)

const (
	esc        = "\x1b"
	csi        = "\x1b["
	ss3        = "\x1bO"
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

var cursorKeys = map[Key]byte{
	KeyUp: 'A', KeyDown: 'B', KeyRight: 'C', KeyLeft: 'D', KeyHome: 'H', KeyEnd: 'F',
	KeyF1: 'P', KeyF2: 'Q', KeyF3: 'R', KeyF4: 'S',
}

var tildeKeys = map[Key]int{
	KeyInsert: 2, KeyDelete: 3, KeyPageUp: 5, KeyPageDown: 6,
	KeyF5: 15, KeyF6: 17, KeyF7: 18, KeyF8: 19, KeyF9: 20, KeyF10: 21, KeyF11: 23, KeyF12: 24,
}

func modParam(m Mod) int {
	return 1 + int(m&(ModShift|ModAlt|ModCtrl))
}

func EncodeKey(k Key, m Mod, md vt.Modes) []byte {
	if final, ok := cursorKeys[k]; ok {
		return encodeCursorKey(k, final, m, md)
	}

	if n, ok := tildeKeys[k]; ok {
		if m == 0 {
			return fmt.Appendf(nil, "%s%d~", csi, n)
		}

		return fmt.Appendf(nil, "%s%d;%d~", csi, n, modParam(m))
	}

	var out string

	switch k {
	case KeyEnter:
		out = "\r"
	case KeyTab:
		if m&ModShift != 0 {
			return []byte(csi + "Z")
		}

		out = "\t"
	case KeyBackspace:
		out = "\x7f"

		if m&ModCtrl != 0 {
			out = "\x08"
		}
	case KeyEscape:
		out = esc
	default:
		return nil
	}

	if m&ModAlt != 0 {
		out = esc + out
	}

	return []byte(out)
}

func encodeCursorKey(k Key, final byte, m Mod, md vt.Modes) []byte {
	if m != 0 {
		return fmt.Appendf(nil, "%s1;%d%c", csi, modParam(m), final)
	}

	isFn := k >= KeyF1 && k <= KeyF4

	if isFn || md.CursorKeysApp {
		return []byte(ss3 + string(final))
	}

	return []byte(csi + string(final))
}

func EncodeRune(r rune, m Mod) []byte {
	s := string(r)

	if m&ModCtrl != 0 {
		if c, ok := ctrlByte(r); ok {
			s = string(c)
		}
	}

	if m&ModAlt != 0 {
		s = esc + s
	}

	return []byte(s)
}

func ctrlByte(r rune) (byte, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return byte(r - 'a' + 1), true
	case r >= '@' && r <= '_':
		return byte(r - '@'), true
	case r == ' ' || r == '2':
		return 0, true
	case r >= '3' && r <= '7':
		return byte(r - '3' + 0x1b), true
	case r == '8' || r == '?':
		return 0x7f, true
	case r == '/':
		return 0x1f, true
	}

	return 0, false
}

func EncodePaste(s string, md vt.Modes) []byte {
	s = strings.ReplaceAll(s, "\r\n", "\r")
	s = strings.ReplaceAll(s, "\n", "\r")

	if !md.BracketedPaste {
		return []byte(s)
	}

	s = strings.Map(func(r rune) rune {
		if r == 0x1b || (r >= 0x80 && r <= 0x9f) {
			return -1
		}

		return r
	}, s)

	return []byte(pasteStart + s + pasteEnd)
}

func EncodeFocus(focused bool, md vt.Modes) []byte {
	if !md.FocusEvents {
		return nil
	}

	if focused {
		return []byte(csi + "I")
	}

	return []byte(csi + "O")
}

type MouseButton uint8

const (
	MouseLeft MouseButton = iota
	MouseMiddle
	MouseRight
	MouseRelease
	MouseWheelUp   = 64
	MouseWheelDown = 65
)

type MouseAction uint8

const (
	MousePress MouseAction = iota
	MouseUp
	MouseMove
)

const mouseMotionFlag = 32

func EncodeMouse(b MouseButton, action MouseAction, x, y int, m Mod, md vt.Modes) []byte {
	switch md.Mouse {
	case vt.MouseNone:
		return nil
	case vt.MouseClick:
		if action == MouseMove {
			return nil
		}
	case vt.MouseDrag:
		if action == MouseMove && b == MouseRelease {
			return nil
		}
	}

	code := int(b)

	if action == MouseMove {
		code += mouseMotionFlag
	}

	if m&ModShift != 0 {
		code += 4
	}

	if m&ModAlt != 0 {
		code += 8
	}

	if m&ModCtrl != 0 {
		code += 16
	}

	if md.MouseSGR {
		final := 'M'

		if action == MouseUp {
			final = 'm'
		}

		return fmt.Appendf(nil, "%s<%d;%d;%d%c", csi, code, x+1, y+1, final)
	}

	if action == MouseUp {
		code = int(MouseRelease)
	}

	if x > 222 || y > 222 {
		return nil
	}

	return []byte{0x1b, '[', 'M', byte(32 + code), byte(33 + x), byte(33 + y)}
}
