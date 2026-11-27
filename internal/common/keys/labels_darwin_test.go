//go:build darwin && !ios

package keys

var wantLabels = map[string]string{
	"Ctrl+Shift+T":              "⌘T",
	"Ctrl+Tab / Ctrl+Shift+Tab": "⌃Tab / ⌃⇧Tab",
	"Ctrl+Alt+S":                "⌘,",
	"Alt+1 … Alt+6":             "⌥1 … ⌥6",
}
