//go:build darwin && cgo

package urlhandler

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <stdlib.h>
#include <CoreServices/CoreServices.h>

#pragma clang diagnostic ignored "-Wdeprecated-declarations"

static int pl_is_default(const char *scheme, const char *bundle) {
	CFStringRef s = CFStringCreateWithCString(NULL, scheme, kCFStringEncodingUTF8);
	CFStringRef cur = LSCopyDefaultHandlerForURLScheme(s);
	CFRelease(s);

	if (cur == NULL) {
		return 0;
	}

	CFStringRef b = CFStringCreateWithCString(NULL, bundle, kCFStringEncodingUTF8);
	int same = CFStringCompare(cur, b, kCFCompareCaseInsensitive) == kCFCompareEqualTo;
	CFRelease(b);
	CFRelease(cur);
	return same;
}

static int pl_set_default(const char *scheme, const char *bundle) {
	CFStringRef s = CFStringCreateWithCString(NULL, scheme, kCFStringEncodingUTF8);
	CFStringRef b = CFStringCreateWithCString(NULL, bundle, kCFStringEncodingUTF8);
	OSStatus st = LSSetDefaultHandlerForURLScheme(s, b);
	CFRelease(s);
	CFRelease(b);
	return (int)st;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

const (
	bundleID   = "io.github.borealssh.boreal"
	fallbackID = "com.apple.Terminal"
)

var schemes = []string{"ssh", "sftp"}

var errNoBundle = errors.New("Links can only be registered when Boreal runs from Boreal.app")

func Registered() bool {
	for _, s := range schemes {
		if !isDefault(s, bundleID) {
			return false
		}
	}

	return true
}

func Set(enabled bool) error {
	for _, s := range schemes {
		if !enabled && !isDefault(s, bundleID) {
			continue
		}

		target := bundleID

		if !enabled {
			target = fallbackID
		}

		if st := setDefault(s, target); st != 0 {
			if enabled {
				return fmt.Errorf("%w (status %d)", errNoBundle, st)
			}

			return fmt.Errorf("urlhandler: release %s:// links: status %d", s, st)
		}
	}

	return nil
}

func isDefault(scheme, bundle string) bool {
	cs, cb := C.CString(scheme), C.CString(bundle)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cb))
	return C.pl_is_default(cs, cb) != 0
}

func setDefault(scheme, bundle string) int {
	cs, cb := C.CString(scheme), C.CString(bundle)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cb))
	return int(C.pl_set_default(cs, cb))
}
