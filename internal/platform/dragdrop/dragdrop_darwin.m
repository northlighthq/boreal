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

- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)p operation:(NSDragOperation)op {
	if (op == NSDragOperationNone) {
		borealPromiseDone(self.handle);
	}
}

@end

static NSMutableSet *pl_live;

// borealDragPromise starts a drag of one promised file or folder. It returns 0 when the
// drag started, 1 when the mouse button is already up.
int borealDragPromise(uintptr_t view, uintptr_t handle, const char *name, int dir) {
	if (([NSEvent pressedMouseButtons] & 1) == 0) {
		return 1;
	}

	NSView *v = (__bridge NSView *)(void *)view;
	NSWindow *w = v.window;

	if (w == nil) {
		return 1;
	}

	if (pl_live == nil) {
		pl_live = [NSMutableSet set];
	}

	BorealPromise *p = [BorealPromise new];
	p.handle = handle;
	p.name = [NSString stringWithUTF8String:name];
	p.queue = [NSOperationQueue new];
	[pl_live addObject:p];

	NSString *type = dir ? (NSString *)@"public.folder" : (NSString *)@"public.data";
	NSFilePromiseProvider *provider = [[NSFilePromiseProvider alloc] initWithFileType:type delegate:p];
	NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:provider];

	NSPoint at = [w mouseLocationOutsideOfEventStream];
	NSPoint local = [v convertPoint:at fromView:nil];
	NSImage *icon = dir ? [NSImage imageNamed:NSImageNameFolder] : [[NSWorkspace sharedWorkspace] iconForFileType:p.name.pathExtension];
	icon.size = NSMakeSize(48, 48);
	[item setDraggingFrame:NSMakeRect(local.x - 24, local.y - 24, 48, 48) contents:icon];

	NSEvent *ev = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDragged
	                                 location:at
	                            modifierFlags:0
	                                timestamp:[NSProcessInfo processInfo].systemUptime
	                             windowNumber:w.windowNumber
	                                  context:nil
	                              eventNumber:0
	                               clickCount:1
	                                 pressure:1];

	[v beginDraggingSessionWithItems:@[item] event:ev source:p];
	return 0;
}

void borealReleasePromise(uintptr_t handle) {
	for (BorealPromise *p in [pl_live copy]) {
		if (p.handle == handle) {
			[pl_live removeObject:p];
		}
	}
}
