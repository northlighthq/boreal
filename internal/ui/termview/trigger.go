package termview

import (
	"bytes"

	"gioui.org/io/key"
	"github.com/northlighthq/vt/input"
)

const maxWord = 64

func (v *View) typeText(text string) {
	if v.Expand == nil {
		v.send([]byte(text))
		return
	}

	var out []byte

	for _, r := range text {
		switch {
		case r == ' ' || r == '\t':
			if exp, ok := v.expandWord(); ok {
				v.send(out)
				v.send(exp)
				out = nil
				continue
			}

			v.word = v.word[:0]
		case r < ' ' || r == 0x7f:
			v.word = v.word[:0]
		case len(v.word) < maxWord:
			v.word = append(v.word, r)
		default:
			v.word = v.word[:0]
		}

		out = append(out, string(r)...)
	}

	v.send(out)
}

func (v *View) expandWord() ([]byte, bool) {
	if v.Expand == nil || len(v.word) < 2 {
		return nil, false
	}

	cmd, ok := v.Expand(string(v.word))

	if !ok {
		return nil, false
	}

	erase := bytes.Repeat(input.EncodeKey(input.KeyBackspace, 0, v.modes), len(v.word))
	v.word = v.word[:0]
	return append(erase, cmd...), true
}

func (v *View) trackKey(e key.Event) bool {
	switch {
	case e.Name == key.NameTab && e.Modifiers == 0:
		if exp, ok := v.expandWord(); ok {
			v.send(exp)
			return true
		}

		v.word = v.word[:0]
	case e.Name == key.NameDeleteBackward && e.Modifiers == 0:
		if n := len(v.word); n > 0 {
			v.word = v.word[:n-1]
		}
	default:
		_, special := specialKeys[e.Name]

		if special || e.Modifiers.Contain(key.ModCtrl) || e.Modifiers.Contain(key.ModAlt) || e.Modifiers.Contain(key.ModCommand) {
			v.word = v.word[:0]
		}
	}

	return false
}
