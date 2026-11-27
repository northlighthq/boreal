//go:build linux && !android && !nox11 && cgo

#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>

#include "_cgo_export.h"

#define XDND_VERSION 5

static Display *pl_dpy;
static Window pl_proxy, pl_target, pl_source;
static Atom a_aware, a_proxy, a_enter, a_position, a_status, a_leave, a_drop, a_finished, a_selection, a_copy, a_uris, a_prop;
static int pl_accept;

static void pl_send(Window to, Atom type, long l0, long l1, long l2, long l3, long l4) {
	XEvent ev;
	memset(&ev, 0, sizeof(ev));
	ev.xclient.type = ClientMessage;
	ev.xclient.display = pl_dpy;
	ev.xclient.window = to;
	ev.xclient.message_type = type;
	ev.xclient.format = 32;
	ev.xclient.data.l[0] = l0;
	ev.xclient.data.l[1] = l1;
	ev.xclient.data.l[2] = l2;
	ev.xclient.data.l[3] = l3;
	ev.xclient.data.l[4] = l4;
	XSendEvent(pl_dpy, to, False, NoEventMask, &ev);
	XFlush(pl_dpy);
}

static int pl_has_uris(XClientMessageEvent *e) {
	if ((e->data.l[1] & 1) == 0) {
		for (int i = 2; i < 5; i++) {
			if ((Atom)e->data.l[i] == a_uris) {
				return 1;
			}
		}

		return 0;
	}

	Atom type;
	int format;
	unsigned long n, after;
	unsigned char *data = NULL;
	int found = 0;

	if (XGetWindowProperty(pl_dpy, (Window)e->data.l[0], XInternAtom(pl_dpy, "XdndTypeList", False), 0, 1024, False, XA_ATOM, &type, &format, &n, &after, &data) == Success && data != NULL) {
		Atom *atoms = (Atom *)data;

		for (unsigned long i = 0; i < n; i++) {
			if (atoms[i] == a_uris) {
				found = 1;
			}
		}

		XFree(data);
	}

	return found;
}

static void pl_selection(XSelectionEvent *e) {
	if (e->property == None) {
		pl_send(pl_source, a_finished, pl_target, 0, None, 0, 0);
		return;
	}

	Atom type;
	int format;
	unsigned long n, after;
	unsigned char *data = NULL;

	if (XGetWindowProperty(pl_dpy, pl_proxy, a_prop, 0, 1 << 18, True, AnyPropertyType, &type, &format, &n, &after, &data) == Success && data != NULL) {
		borealX11Drop((char *)data, (int)n);
		XFree(data);
	}

	pl_send(pl_source, a_finished, pl_target, 1, a_copy, 0, 0);
}

static void pl_message(XClientMessageEvent *e) {
	if (e->message_type == a_enter) {
		pl_source = (Window)e->data.l[0];
		pl_accept = pl_has_uris(e);
	} else if (e->message_type == a_position) {
		pl_source = (Window)e->data.l[0];
		pl_send(pl_source, a_status, pl_target, pl_accept ? 1 : 0, 0, 0, pl_accept ? a_copy : None);

		if (pl_accept) {
			borealDragHover(1);
		}
	} else if (e->message_type == a_leave) {
		borealDragHover(0);
	} else if (e->message_type == a_drop) {
		borealDragHover(0);
		pl_source = (Window)e->data.l[0];

		if (!pl_accept) {
			pl_send(pl_source, a_finished, pl_target, 0, None, 0, 0);
			return;
		}

		XConvertSelection(pl_dpy, a_selection, a_uris, a_prop, pl_proxy, (Time)e->data.l[2]);
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

	Atom version = XDND_VERSION;
	XChangeProperty(pl_dpy, pl_target, a_aware, XA_ATOM, 32, PropModeReplace, (unsigned char *)&version, 1);
	XChangeProperty(pl_dpy, pl_proxy, a_aware, XA_ATOM, 32, PropModeReplace, (unsigned char *)&version, 1);
	XChangeProperty(pl_dpy, pl_target, a_proxy, XA_WINDOW, 32, PropModeReplace, (unsigned char *)&pl_proxy, 1);
	XChangeProperty(pl_dpy, pl_proxy, a_proxy, XA_WINDOW, 32, PropModeReplace, (unsigned char *)&pl_proxy, 1);
	XFlush(pl_dpy);
	return 0;
}

void borealX11Loop(void) {
	XEvent ev;

	for (;;) {
		XNextEvent(pl_dpy, &ev);

		switch (ev.type) {
		case ClientMessage:
			pl_message(&ev.xclient);
			break;
		case SelectionNotify:
			pl_selection(&ev.xselection);
			break;
		}
	}
}
