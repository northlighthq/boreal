//go:build linux && !android

package sound

const (
	bellOGA  = "/usr/share/sounds/freedesktop/stereo/bell.oga"
	alsaWAV  = "/usr/share/sounds/alsa/Front_Center.wav"
	appTitle = "Boreal"
)

var candidates = []player{
	{cmd: "canberra-gtk-play", args: []string{"-i", "bell", "-d", appTitle}},
	{cmd: "paplay", file: bellOGA},
	{cmd: "pw-play", file: bellOGA},
	{cmd: "aplay", args: []string{"-q"}, file: alsaWAV},
}
