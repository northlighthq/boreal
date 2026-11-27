//go:build darwin && cgo

package urlhandler

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <stdlib.h>
#include <CoreServices/CoreServices.h>

#pragma clang diagnostic ignored "-Wdeprecated-declarations"

static int pl_is_default(const char *scheme, const char *bundle) {
	CFStringRef schemeRef = CFStringCreateWithCString(NULL, scheme, kCFStringEncodingUTF8);
	CFStringRef current = LSCopyDefaultHandlerForURLScheme(schemeRef);
	CFRelease(schemeRef);

	if (current == NULL) {
		return 0;
	}

	CFStringRef bundleRef = CFStringCreateWithCString(NULL, bundle, kCFStringEncodingUTF8);
	int same = CFStringCompare(current, bundleRef, kCFCompareCaseInsensitive) == kCFCompareEqualTo;
	CFRelease(bundleRef);
	CFRelease(current);
	return same;
}

static int pl_set_default(const char *scheme, const char *bundle) {
	CFStringRef schemeRef = CFStringCreateWithCString(NULL, scheme, kCFStringEncodingUTF8);
	CFStringRef bundleRef = CFStringCreateWithCString(NULL, bundle, kCFStringEncodingUTF8);
	OSStatus status = LSSetDefaultHandlerForURLScheme(schemeRef, bundleRef);
	CFRelease(schemeRef);
	CFRelease(bundleRef);
	return (int)status;
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
	for _, scheme := range schemes {
		if !isDefault(scheme, bundleID) {
			return false
		}
	}

	return true
}

func Set(enabled bool) error {
	for _, scheme := range schemes {
		if !enabled && !isDefault(scheme, bundleID) {
			continue
		}

		target := bundleID

		if !enabled {
			target = fallbackID
		}

		if status := setDefault(scheme, target); status != 0 {
			if enabled {
				return fmt.Errorf("%w (status %d)", errNoBundle, status)
			}

			return fmt.Errorf("urlhandler: release %scheme:// links: status %d", scheme, status)
		}
	}

	return nil
}

func isDefault(scheme, bundle string) bool {
	cScheme, cBundle := C.CString(scheme), C.CString(bundle)
	defer C.free(unsafe.Pointer(cScheme))
	defer C.free(unsafe.Pointer(cBundle))
	return C.pl_is_default(cScheme, cBundle) != 0
}

func setDefault(scheme, bundle string) int {
	cScheme, cBundle := C.CString(scheme), C.CString(bundle)
	defer C.free(unsafe.Pointer(cScheme))
	defer C.free(unsafe.Pointer(cBundle))
	return int(C.pl_set_default(cScheme, cBundle))
}
