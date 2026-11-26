//go:build darwin && cgo

package hello

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework LocalAuthentication -framework Foundation
#include <stdlib.h>
#import <LocalAuthentication/LocalAuthentication.h>

enum { plOK = 0, plUnavailable = 1, plCanceled = 2, plFailed = 3 };

static int pl_available(void) {
	LAContext *ctx = [[LAContext alloc] init];
	return [ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:nil] ? plOK : plUnavailable;
}

static int pl_verify(const char *reason) {
	LAContext *ctx = [[LAContext alloc] init];
	NSError *err = nil;

	if (![ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&err]) {
		return plUnavailable;
	}

	__block int result = plFailed;
	dispatch_semaphore_t done = dispatch_semaphore_create(0);

	[ctx evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
	    localizedReason:[NSString stringWithUTF8String:reason]
	              reply:^(BOOL ok, NSError *e) {
		if (ok) {
			result = plOK;
		} else if (e.code == LAErrorUserCancel || e.code == LAErrorSystemCancel || e.code == LAErrorAppCancel) {
			result = plCanceled;
		}

		dispatch_semaphore_signal(done);
	}];

	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	return result;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

const Name = "Touch ID"

func Available() bool {
	return int(C.pl_available()) == int(C.plOK)
}

// Verify asks for Touch ID. macOS shows its own sheet, so the window handle isn't needed.
func Verify(hwnd uintptr, message string) error {
	reason := C.CString(strings.TrimSpace(message))
	defer C.free(unsafe.Pointer(reason))

	switch int(C.pl_verify(reason)) {
	case int(C.plOK):
		return nil
	case int(C.plUnavailable):
		return ErrUnavailable
	case int(C.plCanceled):
		return ErrCanceled
	default:
		return ErrFailed
	}
}
