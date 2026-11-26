//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <objc/runtime.h>

#include "_cgo_export.h"

static NSDragOperation pl_entered(id self, SEL _cmd, id<NSDraggingInfo> info) {
	borealDragHover(1);
	return NSDragOperationCopy;
}

static NSDragOperation pl_updated(id self, SEL _cmd, id<NSDraggingInfo> info) {
	return NSDragOperationCopy;
}

static void pl_exited(id self, SEL _cmd, id<NSDraggingInfo> info) {
	borealDragHover(0);
}

static BOOL pl_prepare(id self, SEL _cmd, id<NSDraggingInfo> info) {
	return YES;
}

static BOOL pl_perform(id self, SEL _cmd, id<NSDraggingInfo> info) {
	borealDragHover(0);
	NSArray<NSURL *> *urls = [[info draggingPasteboard] readObjectsForClasses:@[[NSURL class]]
	                                                                  options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];

	if (urls.count == 0) {
		return NO;
	}

	NSMutableString *joined = [NSMutableString string];

	for (NSURL *u in urls) {
		[joined appendString:u.path];
		[joined appendString:@"\n"];
	}

	borealDrop((char *)joined.UTF8String);
	return YES;
}

static void pl_add(Class c, SEL sel, IMP imp, const char *types) {
	if (!class_addMethod(c, sel, imp, types)) {
		class_replaceMethod(c, sel, imp, types);
	}
}

void borealEnableDrop(uintptr_t view) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSView *v = (__bridge NSView *)(void *)view;
		Class c = object_getClass(v);
		pl_add(c, @selector(draggingEntered:), (IMP)pl_entered, "Q@:@");
		pl_add(c, @selector(draggingUpdated:), (IMP)pl_updated, "Q@:@");
		pl_add(c, @selector(draggingExited:), (IMP)pl_exited, "v@:@");
		pl_add(c, @selector(prepareForDragOperation:), (IMP)pl_prepare, "c@:@");
		pl_add(c, @selector(performDragOperation:), (IMP)pl_perform, "c@:@");
		[v registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
	});
}
