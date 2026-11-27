//go:build darwin && cgo

package sessionlock

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>

static int pl_screen_locked(void) {
	CFDictionaryRef session = CGSessionCopyCurrentDictionary();

	if (session == NULL) {
		return 0;
	}

	int locked = 0;
	CFBooleanRef value = CFDictionaryGetValue(session, CFSTR("CGSSessionScreenIsLocked"));

	if (value != NULL && CFGetTypeID(value) == CFBooleanGetTypeID()) {
		locked = CFBooleanGetValue(value);
	}

	CFRelease(session);
	return locked;
}
*/
import "C"

func Locked() bool {
	return C.pl_screen_locked() != 0
}
