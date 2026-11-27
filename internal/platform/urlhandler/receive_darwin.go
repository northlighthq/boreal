//go:build darwin && cgo

package urlhandler

/*
#cgo LDFLAGS: -framework AppKit -framework Foundation
*/
import "C"

import "github.com/northlighthq/boreal/internal/platform/instance"

//export borealOpenURL
func borealOpenURL(link *C.char) {
	instance.Deliver([]string{C.GoString(link)})
}
