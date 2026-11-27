//go:build linux && !android && !nox11 && cgo

#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/select.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>

#include "_cgo_export.h"

#define XDND_VERSION 5
#define DRAG_TICK_USEC 16000
#define FIND_DEPTH 8
#define COORD_MASK 0xffff

enum { dragIdle, dragMoving, dragDropped };

static Display *pl_dpy;
static Window pl_proxy, pl_target, pl_source;
static Atom a_aware, a_proxy, a_enter, a_position, a_status, a_leave, a_drop, a_finished, a_selection, a_copy, a_uris, a_prop;
static int pl_accept;
static int pl_wake[2] = {-1, -1};
static int pl_drag_state;
static Window pl_drag_target, pl_drag_dest;
static int pl_drag_accepted;
static Atom a_targets;

static void pl_send_as(Window to, Window window, Atom type, long l0, long l1, long l2, long l3, long l4) {
	XEvent event;
	memset(&event, 0, sizeof(event));
	event.xclient.type = ClientMessage;
	event.xclient.display = pl_dpy;
	event.xclient.window = window;
	event.xclient.message_type = type;
	event.xclient.format = 32;
	event.xclient.data.l[0] = l0;
	event.xclient.data.l[1] = l1;
	event.xclient.data.l[2] = l2;
	event.xclient.data.l[3] = l3;
	event.xclient.data.l[4] = l4;
	XSendEvent(pl_dpy, to, False, NoEventMask, &event);
	XFlush(pl_dpy);
}

static void pl_send(Window to, Atom type, long l0, long l1, long l2, long l3, long l4) {
	pl_send_as(to, to, type, l0, l1, l2, l3, l4);
}

static Window pl_window_property(Window window, Atom property) {
	Atom type;
	int format;
	unsigned long count, after;
	unsigned char *data = NULL;
	Window found = None;

	if (XGetWindowProperty(pl_dpy, window, property, 0, 1, False, AnyPropertyType, &type, &format, &count, &after, &data) == Success && data != NULL) {
		if (count == 1 && format == 32) {
			found = *(Window *)data;
		}

		XFree(data);
	}

	return found;
}

static Window pl_find_aware(int rootX, int rootY) {
	Window root = DefaultRootWindow(pl_dpy);
	Window current = root, child = None;
	int localX, localY;

	for (int depth = 0; depth < FIND_DEPTH; depth++) {
		if (!XTranslateCoordinates(pl_dpy, root, current, rootX, rootY, &localX, &localY, &child) || child == None) {
			return None;
		}

		current = child;

		if (pl_window_property(current, a_aware) != None) {
			return current;
		}
	}

	return None;
}

static void pl_drag_leave(void) {
	if (pl_drag_target != None) {
		pl_send_as(pl_drag_dest, pl_drag_target, a_leave, pl_proxy, 0, 0, 0, 0);
	}

	pl_drag_target = pl_drag_dest = None;
	pl_drag_accepted = 0;
}

static void pl_drag_end(void) {
	pl_drag_state = dragIdle;
	pl_drag_target = pl_drag_dest = None;
	pl_drag_accepted = 0;
}

static void pl_drag_tick(void) {
	Window root, child;
	int rootX, rootY, localX, localY;
	unsigned int mask;

	if (!XQueryPointer(pl_dpy, DefaultRootWindow(pl_dpy), &root, &child, &rootX, &rootY, &localX, &localY, &mask)) {
		pl_drag_leave();
		pl_drag_end();
		return;
	}

	Window target = pl_find_aware(rootX, rootY);

	if (target == pl_target) {
		target = None;
	}

	if (target != pl_drag_target) {
		pl_drag_leave();

		if (target != None) {
			Window proxy = pl_window_property(target, a_proxy);
			pl_drag_target = target;
			pl_drag_dest = proxy != None ? proxy : target;
			pl_send_as(pl_drag_dest, pl_drag_target, a_enter, pl_proxy, (long)XDND_VERSION << 24, a_uris, 0, 0);
		}
	}

	if ((mask & Button1Mask) == 0) {
		if (pl_drag_target != None && pl_drag_accepted) {
			pl_send_as(pl_drag_dest, pl_drag_target, a_drop, pl_proxy, 0, CurrentTime, 0, 0);
			pl_drag_state = dragDropped;
			return;
		}

		pl_drag_leave();
		pl_drag_end();
		return;
	}

	if (pl_drag_target != None) {
		pl_send_as(pl_drag_dest, pl_drag_target, a_position, pl_proxy, 0, ((long)rootX << 16) | (rootY & COORD_MASK), CurrentTime, a_copy);
	}
}

static void pl_serve(XSelectionRequestEvent *request) {
	XEvent reply;
	memset(&reply, 0, sizeof(reply));
	reply.xselection.type = SelectionNotify;
	reply.xselection.requestor = request->requestor;
	reply.xselection.selection = request->selection;
	reply.xselection.target = request->target;
	reply.xselection.time = request->time;
	reply.xselection.property = None;

	if (request->target == a_targets) {
		Atom offered[] = {a_targets, a_uris};
		XChangeProperty(pl_dpy, request->requestor, request->property, XA_ATOM, 32, PropModeReplace, (unsigned char *)offered, 2);
		reply.xselection.property = request->property;
	} else if (request->target == a_uris) {
		char *uris = borealX11URIs();

		if (uris != NULL) {
			XChangeProperty(pl_dpy, request->requestor, request->property, a_uris, 8, PropModeReplace, (unsigned char *)uris, strlen(uris));
			reply.xselection.property = request->property;
			free(uris);
		}
	}

	XSendEvent(pl_dpy, request->requestor, False, NoEventMask, &reply);
	XFlush(pl_dpy);
}

static int pl_has_uris(XClientMessageEvent *message) {
	if ((message->data.l[1] & 1) == 0) {
		for (int index = 2; index < 5; index++) {
			if ((Atom)message->data.l[index] == a_uris) {
				return 1;
			}
		}

		return 0;
	}

	Atom type;
	int format;
	unsigned long count, after;
	unsigned char *data = NULL;
	int found = 0;

	if (XGetWindowProperty(pl_dpy, (Window)message->data.l[0], XInternAtom(pl_dpy, "XdndTypeList", False), 0, 1024, False, XA_ATOM, &type, &format, &count, &after, &data) == Success && data != NULL) {
		Atom *atoms = (Atom *)data;

		for (unsigned long index = 0; index < count; index++) {
			if (atoms[index] == a_uris) {
				found = 1;
			}
		}

		XFree(data);
	}

	return found;
}

static void pl_selection(XSelectionEvent *selection) {
	if (selection->property == None) {
		pl_send(pl_source, a_finished, pl_target, 0, None, 0, 0);
		return;
	}

	Atom type;
	int format;
	unsigned long count, after;
	unsigned char *data = NULL;

	if (XGetWindowProperty(pl_dpy, pl_proxy, a_prop, 0, 1 << 18, True, AnyPropertyType, &type, &format, &count, &after, &data) == Success && data != NULL) {
		borealX11Drop((char *)data, (int)count);
		XFree(data);
	}

	pl_send(pl_source, a_finished, pl_target, 1, a_copy, 0, 0);
}

static void pl_message(XClientMessageEvent *message) {
	if (message->message_type == a_status && pl_drag_state != dragIdle) {
		if ((Window)message->data.l[0] == pl_drag_target) {
			pl_drag_accepted = message->data.l[1] & 1;
		}
	} else if (message->message_type == a_finished && pl_drag_state == dragDropped) {
		pl_drag_end();
	} else if (message->message_type == a_enter) {
		pl_source = (Window)message->data.l[0];
		pl_accept = pl_has_uris(message);
	} else if (message->message_type == a_position) {
		pl_source = (Window)message->data.l[0];
		pl_send(pl_source, a_status, pl_target, pl_accept ? 1 : 0, 0, 0, pl_accept ? a_copy : None);

		if (pl_accept) {
			borealDragHover(1);
		}
	} else if (message->message_type == a_leave) {
		borealDragHover(0);
	} else if (message->message_type == a_drop) {
		borealDragHover(0);
		pl_source = (Window)message->data.l[0];

		if (!pl_accept) {
			pl_send(pl_source, a_finished, pl_target, 0, None, 0, 0);
			return;
		}

		XConvertSelection(pl_dpy, a_selection, a_uris, a_prop, pl_proxy, (Time)message->data.l[2]);
		XFlush(pl_dpy);
	}
}

int borealX11Setup(unsigned long target) {
	pl_dpy = XOpenDisplay(NULL);

	if (pl_dpy == NULL) {
		return -1;
	}

	pl_target = (Window)target;
	pl_proxy = XCreateSimpleWindow(pl_dpy, DefaultRootWindow(pl_dpy), -10, -10, 1, 1, 0, 0, 0);

	a_aware = XInternAtom(pl_dpy, "XdndAware", False);
	a_proxy = XInternAtom(pl_dpy, "XdndProxy", False);
	a_enter = XInternAtom(pl_dpy, "XdndEnter", False);
	a_position = XInternAtom(pl_dpy, "XdndPosition", False);
	a_status = XInternAtom(pl_dpy, "XdndStatus", False);
	a_leave = XInternAtom(pl_dpy, "XdndLeave", False);
	a_drop = XInternAtom(pl_dpy, "XdndDrop", False);
	a_finished = XInternAtom(pl_dpy, "XdndFinished", False);
	a_selection = XInternAtom(pl_dpy, "XdndSelection", False);
	a_copy = XInternAtom(pl_dpy, "XdndActionCopy", False);
	a_uris = XInternAtom(pl_dpy, "text/uri-list", False);
	a_prop = XInternAtom(pl_dpy, "BOREAL_DROP", False);
	a_targets = XInternAtom(pl_dpy, "TARGETS", False);

	if (pipe(pl_wake) != 0) {
		pl_wake[0] = pl_wake[1] = -1;
	}

	Atom version = XDND_VERSION;
	XChangeProperty(pl_dpy, pl_target, a_aware, XA_ATOM, 32, PropModeReplace, (unsigned char *)&version, 1);
	XChangeProperty(pl_dpy, pl_proxy, a_aware, XA_ATOM, 32, PropModeReplace, (unsigned char *)&version, 1);
	XChangeProperty(pl_dpy, pl_target, a_proxy, XA_WINDOW, 32, PropModeReplace, (unsigned char *)&pl_proxy, 1);
	XChangeProperty(pl_dpy, pl_proxy, a_proxy, XA_WINDOW, 32, PropModeReplace, (unsigned char *)&pl_proxy, 1);
	XFlush(pl_dpy);
	return 0;
}

static Display *pl_query_dpy;

static int pl_button_held(void) {
	if (pl_query_dpy == NULL) {
		pl_query_dpy = XOpenDisplay(NULL);
	}

	if (pl_query_dpy == NULL) {
		return 1;
	}

	Window root, child;
	int rootX, rootY, localX, localY;
	unsigned int mask;

	if (!XQueryPointer(pl_query_dpy, DefaultRootWindow(pl_query_dpy), &root, &child, &rootX, &rootY, &localX, &localY, &mask)) {
		return 1;
	}

	return (mask & Button1Mask) != 0;
}

int borealX11StartDrag(void) {
	if (pl_wake[1] < 0 || !pl_button_held()) {
		return -1;
	}

	char wake = 1;
	return write(pl_wake[1], &wake, 1) == 1 ? 0 : -1;
}

static void pl_begin_drag(void) {
	pl_drag_leave();
	XSetSelectionOwner(pl_dpy, a_selection, pl_proxy, CurrentTime);
	pl_drag_state = dragMoving;
	pl_drag_tick();
}

void borealX11Loop(void) {
	XEvent event;
	int connection = ConnectionNumber(pl_dpy);

	for (;;) {
		while (XPending(pl_dpy)) {
			XNextEvent(pl_dpy, &event);

			switch (event.type) {
			case ClientMessage:
				pl_message(&event.xclient);
				break;
			case SelectionNotify:
				pl_selection(&event.xselection);
				break;
			case SelectionRequest:
				pl_serve(&event.xselectionrequest);
				break;
			}
		}

		fd_set ready;
		FD_ZERO(&ready);
		FD_SET(connection, &ready);
		int highest = connection;

		if (pl_wake[0] >= 0) {
			FD_SET(pl_wake[0], &ready);
			highest = pl_wake[0] > highest ? pl_wake[0] : highest;
		}

		struct timeval tick = {0, DRAG_TICK_USEC};

		if (select(highest + 1, &ready, NULL, NULL, pl_drag_state == dragMoving ? &tick : NULL) < 0) {
			continue;
		}

		if (pl_wake[0] >= 0 && FD_ISSET(pl_wake[0], &ready)) {
			char wake;

			if (read(pl_wake[0], &wake, 1) == 1) {
				pl_begin_drag();
			}
		}

		if (pl_drag_state == dragMoving) {
			pl_drag_tick();
		}
	}
}
