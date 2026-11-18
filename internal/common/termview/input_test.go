package termview

import (
	"testing"

	"gioui.org/io/key"
)

func TestWordKey(t *testing.T) {
	cases := []struct {
		name   string
		key    key.Name
		mods   key.Modifiers
		conpty bool
		alt    bool
		want   string
		ok     bool
	}{
		{name: "ctrl backspace", key: key.NameDeleteBackward, mods: key.ModCtrl, want: wordBack, ok: true},
		{name: "ctrl delete", key: key.NameDeleteForward, mods: key.ModCtrl, want: wordForward, ok: true},
		{name: "plain backspace", key: key.NameDeleteBackward},
		{name: "ctrl shift backspace", key: key.NameDeleteBackward, mods: key.ModCtrl | key.ModShift},
		{name: "conpty keeps native keys", key: key.NameDeleteBackward, mods: key.ModCtrl, conpty: true},
		{name: "full screen apps keep native keys", key: key.NameDeleteBackward, mods: key.ModCtrl, alt: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := &View{ConPTY: c.conpty}
			v.modes.AltScreen = c.alt
			got, ok := v.wordKey(key.Event{Name: c.key, Modifiers: c.mods})

			if got != c.want || ok != c.ok {
				t.Fatalf("wordKey = %q, %v; want %q, %v", got, ok, c.want, c.ok)
			}
		})
	}
}
