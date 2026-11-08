package sound

import "golang.org/x/sys/windows"

const (
	Available      = true
	mbIconAsterisk = 0x40
)

var messageBeep = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBeep")

func Play() {
	messageBeep.Call(mbIconAsterisk)
}
