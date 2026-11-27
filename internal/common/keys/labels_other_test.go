//go:build !darwin || ios

package keys

var wantLabels = map[string]string{
	"Ctrl+Shift+T":              "Ctrl+Shift+T",
	"Ctrl+Tab / Ctrl+Shift+Tab": "Ctrl+Tab / Ctrl+Shift+Tab",
	"Ctrl+Alt+S":                "Ctrl+Alt+S",
	"Alt+1 … Alt+6":             "Alt+1 … Alt+6",
}
