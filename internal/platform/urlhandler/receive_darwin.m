//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <objc/runtime.h>

#include "_cgo_export.h"

static void pl_open_urls(id self, SEL selector, NSApplication *application, NSArray<NSURL *> *urls) {
	for (NSURL *link in urls) {
		borealOpenURL((char *)link.absoluteString.UTF8String);
	}
}

__attribute__((constructor)) static void pl_take_open_urls(void) {
	Class delegate = objc_getClass("GioAppDelegate");

	if (delegate == Nil) {
		return;
	}

	class_replaceMethod(delegate, @selector(application:openURLs:), (IMP)pl_open_urls, "v@:@@");
}
