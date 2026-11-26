//go:build darwin && cgo

package sessionlock

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>

static int pl_screen_locked(void) {
	CFDictionaryRef d = CGSessionCopyCurrentDictionary();

	if (d == NULL) {
		return 0;
	}

	int locked = 0;
	CFBooleanRef v = CFDictionaryGetValue(d, CFSTR("CGSSessionScreenIsLocked"));

	if (v != NULL && CFGetTypeID(v) == CFBooleanGetTypeID()) {
		locked = CFBooleanGetValue(v);
	}

	CFRelease(d);
	return locked;
}
*/
import "C"

func Locked() bool {
	return C.pl_screen_locked() != 0
}
