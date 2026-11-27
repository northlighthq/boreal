//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <objc/runtime.h>

#include <stdlib.h>

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

	for (NSURL *fileURL in urls) {
		[joined appendString:fileURL.path];
		[joined appendString:@"\n"];
	}

	borealDrop((char *)joined.UTF8String);
	return YES;
}

static void pl_add(Class viewClass, SEL sel, IMP imp, const char *types) {
	if (!class_addMethod(viewClass, sel, imp, types)) {
		class_replaceMethod(viewClass, sel, imp, types);
	}
}

void borealEnableDrop(uintptr_t view) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSView *gioView = (__bridge NSView *)(void *)view;
		Class viewClass = object_getClass(gioView);
		pl_add(viewClass, @selector(draggingEntered:), (IMP)pl_entered, "Q@:@");
		pl_add(viewClass, @selector(draggingUpdated:), (IMP)pl_updated, "Q@:@");
		pl_add(viewClass, @selector(draggingExited:), (IMP)pl_exited, "gioView@:@");
		pl_add(viewClass, @selector(prepareForDragOperation:), (IMP)pl_prepare, "viewClass@:@");
		pl_add(viewClass, @selector(performDragOperation:), (IMP)pl_perform, "viewClass@:@");
		[gioView registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
	});
}

@interface BorealPromise : NSObject <NSFilePromiseProviderDelegate, NSDraggingSource>
@property(nonatomic) uintptr_t handle;
@property(nonatomic, copy) NSString *name;
@property(nonatomic, strong) NSOperationQueue *queue;
@end

@implementation BorealPromise

- (NSString *)filePromiseProvider:(NSFilePromiseProvider *)provider fileNameForType:(NSString *)fileType {
	return self.name;
}

- (NSOperationQueue *)operationQueueForFilePromiseProvider:(NSFilePromiseProvider *)provider {
	return self.queue;
}

- (void)filePromiseProvider:(NSFilePromiseProvider *)provider writePromiseToURL:(NSURL *)url completionHandler:(void (^)(NSError *))done {
	char *err = borealWritePromise(self.handle, (char *)url.path.UTF8String);

	if (err == NULL) {
		done(nil);
		return;
	}

	NSString *msg = [NSString stringWithUTF8String:err];
	free(err);
	done([NSError errorWithDomain:@"Boreal" code:1 userInfo:@{NSLocalizedDescriptionKey: msg}]);
}

- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
	return context == NSDraggingContextOutsideApplication ? NSDragOperationCopy : NSDragOperationNone;
}

- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)promise operation:(NSDragOperation)operation {
	if (operation == NSDragOperationNone) {
		borealPromiseDone(self.handle);
	}
}

@end

static NSMutableSet *pl_live;

int borealDragPromise(uintptr_t view, uintptr_t handle, const char *name, int dir) {
	if (([NSEvent pressedMouseButtons] & 1) == 0) {
		return 1;
	}

	NSView *gioView = (__bridge NSView *)(void *)view;
	NSWindow *window = gioView.window;

	if (window == nil) {
		return 1;
	}

	if (pl_live == nil) {
		pl_live = [NSMutableSet set];
	}

	BorealPromise *promise = [BorealPromise new];
	promise.handle = handle;
	promise.name = [NSString stringWithUTF8String:name];
	promise.queue = [NSOperationQueue new];
	[pl_live addObject:promise];

	NSString *type = dir ? (NSString *)@"public.folder" : (NSString *)@"public.data";
	NSFilePromiseProvider *provider = [[NSFilePromiseProvider alloc] initWithFileType:type delegate:promise];
	NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:provider];

	NSPoint at = [window mouseLocationOutsideOfEventStream];
	NSPoint local = [gioView convertPoint:at fromView:nil];
	NSImage *icon = dir ? [NSImage imageNamed:NSImageNameFolder] : [[NSWorkspace sharedWorkspace] iconForFileType:promise.name.pathExtension];
	icon.size = NSMakeSize(48, 48);
	[item setDraggingFrame:NSMakeRect(local.x - 24, local.y - 24, 48, 48) contents:icon];

	NSEvent *dragEvent = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDragged
	                                 location:at
	                            modifierFlags:0
	                                timestamp:[NSProcessInfo processInfo].systemUptime
	                             windowNumber:window.windowNumber
	                                  context:nil
	                              eventNumber:0
	                               clickCount:1
	                                 pressure:1];

	[gioView beginDraggingSessionWithItems:@[item] event:dragEvent source:promise];
	return 0;
}

void borealReleasePromise(uintptr_t handle) {
	for (BorealPromise *promise in [pl_live copy]) {
		if (promise.handle == handle) {
			[pl_live removeObject:promise];
		}
	}
}
