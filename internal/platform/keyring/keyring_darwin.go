//go:build darwin && cgo

package keyring

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <stdlib.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>

static CFMutableDictionaryRef kr_query(const char *service, const char *account) {
	CFMutableDictionaryRef q = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFStringRef s = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef a = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
	CFDictionarySetValue(q, kSecClass, kSecClassGenericPassword);
	CFDictionarySetValue(q, kSecAttrService, s);
	CFDictionarySetValue(q, kSecAttrAccount, a);
	CFRelease(s);
	CFRelease(a);
	return q;
}

static OSStatus kr_set(const char *service, const char *account, const char *label, const void *data, int len) {
	CFMutableDictionaryRef q = kr_query(service, account);
	SecItemDelete(q);
	CFDataRef d = CFDataCreate(NULL, data, len);
	CFStringRef l = CFStringCreateWithCString(NULL, label, kCFStringEncodingUTF8);
	CFDictionarySetValue(q, kSecValueData, d);
	CFDictionarySetValue(q, kSecAttrLabel, l);
	OSStatus st = SecItemAdd(q, NULL);
	CFRelease(d);
	CFRelease(l);
	CFRelease(q);
	return st;
}

static OSStatus kr_get(const char *service, const char *account, void **out, int *len) {
	CFMutableDictionaryRef q = kr_query(service, account);
	CFDictionarySetValue(q, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(q, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef r = NULL;
	OSStatus st = SecItemCopyMatching(q, &r);
	CFRelease(q);

	if (st != errSecSuccess) {
		return st;
	}

	CFDataRef d = (CFDataRef)r;
	*len = (int)CFDataGetLength(d);
	*out = malloc(*len > 0 ? *len : 1);
	memcpy(*out, CFDataGetBytePtr(d), *len);
	CFRelease(r);
	return st;
}

static OSStatus kr_delete(const char *service, const char *account) {
	CFMutableDictionaryRef q = kr_query(service, account);
	OSStatus st = SecItemDelete(q);
	CFRelease(q);
	return st;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const (
	service      = "Boreal"
	itemNotFound = -25300
)

func status(st C.OSStatus) error {
	switch st {
	case 0:
		return nil
	case itemNotFound:
		return ErrNotFound
	}

	return fmt.Errorf("keyring: keychain error %d", int(st))
}

func Set(id, label string, value []byte) error {
	cs, ca, cl := C.CString(service), C.CString(id), C.CString(label)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ca))
	defer C.free(unsafe.Pointer(cl))

	var p unsafe.Pointer

	if len(value) > 0 {
		p = unsafe.Pointer(&value[0])
	}

	return status(C.kr_set(cs, ca, cl, p, C.int(len(value))))
}

func Get(id string) ([]byte, error) {
	cs, ca := C.CString(service), C.CString(id)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ca))

	var out unsafe.Pointer
	var n C.int

	if err := status(C.kr_get(cs, ca, &out, &n)); err != nil {
		return nil, err
	}

	defer C.free(out)

	if n == 0 {
		return nil, ErrNotFound
	}

	return C.GoBytes(out, n), nil
}

func Delete(id string) error {
	cs, ca := C.CString(service), C.CString(id)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ca))

	return status(C.kr_delete(cs, ca))
}
