//go:build linux && !android && !nowayland && cgo

#include <stdlib.h>
#include <string.h>
#include <wayland-client.h>

#include "_cgo_export.h"

#define URI_LIST "text/uri-list"

struct pl_offer {
	int has_uris;
};

static struct wl_event_queue *pl_queue;
static struct wl_surface *pl_surface;
static struct wl_seat *pl_seat;
static struct wl_data_device_manager *pl_manager;
static uint32_t pl_manager_version;
static struct wl_data_offer *pl_current;

static void pl_offer_offer(void *data, struct wl_data_offer *offer, const char *mime) {
	struct pl_offer *o = data;

	if (strcmp(mime, URI_LIST) == 0) {
		o->has_uris = 1;
	}
}

static void pl_offer_source_actions(void *data, struct wl_data_offer *offer, uint32_t actions) {}

static void pl_offer_action(void *data, struct wl_data_offer *offer, uint32_t action) {}

static const struct wl_data_offer_listener pl_offer_listener = {
	.offer = pl_offer_offer,
	.source_actions = pl_offer_source_actions,
	.action = pl_offer_action,
};

static void pl_destroy_offer(struct wl_data_offer *offer) {
	if (offer == NULL) {
		return;
	}

	free(wl_data_offer_get_user_data(offer));
	wl_data_offer_destroy(offer);
}

static void pl_dev_data_offer(void *data, struct wl_data_device *dev, struct wl_data_offer *offer) {
	struct pl_offer *o = calloc(1, sizeof(*o));
	wl_data_offer_add_listener(offer, &pl_offer_listener, o);
}

static void pl_dev_enter(void *data, struct wl_data_device *dev, uint32_t serial, struct wl_surface *surf, wl_fixed_t x, wl_fixed_t y, struct wl_data_offer *offer) {
	if (pl_current != NULL && pl_current != offer) {
		pl_destroy_offer(pl_current);
	}

	pl_current = offer;

	if (offer == NULL) {
		return;
	}

	struct pl_offer *o = wl_data_offer_get_user_data(offer);

	if (surf != pl_surface || o == NULL || !o->has_uris) {
		wl_data_offer_accept(offer, serial, NULL);
		return;
	}

	wl_data_offer_accept(offer, serial, URI_LIST);

	if (pl_manager_version >= 3) {
		wl_data_offer_set_actions(offer, WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY, WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY);
	}

	borealDragHover(1);
}

static void pl_dev_leave(void *data, struct wl_data_device *dev) {
	borealDragHover(0);

	// After a drop, the offer is handed to Go and released there.
	if (pl_current != NULL) {
		pl_destroy_offer(pl_current);
		pl_current = NULL;
	}
}

static void pl_dev_motion(void *data, struct wl_data_device *dev, uint32_t t, wl_fixed_t x, wl_fixed_t y) {}

static void pl_dev_drop(void *data, struct wl_data_device *dev) {
	struct wl_data_offer *offer = pl_current;
	pl_current = NULL;
	borealDragHover(0);

	if (offer == NULL) {
		return;
	}

	struct pl_offer *o = wl_data_offer_get_user_data(offer);

	if (o == NULL || !o->has_uris) {
		pl_destroy_offer(offer);
		return;
	}

	borealWaylandDrop((uintptr_t)offer);
}

static void pl_dev_selection(void *data, struct wl_data_device *dev, struct wl_data_offer *offer) {
	// Gio handles the clipboard through its own data device.
	if (offer != NULL && offer != pl_current) {
		pl_destroy_offer(offer);
	}
}

static const struct wl_data_device_listener pl_dev_listener = {
	.data_offer = pl_dev_data_offer,
	.enter = pl_dev_enter,
	.leave = pl_dev_leave,
	.motion = pl_dev_motion,
	.drop = pl_dev_drop,
	.selection = pl_dev_selection,
};

static void pl_global(void *data, struct wl_registry *reg, uint32_t name, const char *iface, uint32_t version) {
	if (strcmp(iface, wl_seat_interface.name) == 0 && pl_seat == NULL) {
		pl_seat = wl_registry_bind(reg, name, &wl_seat_interface, version < 5 ? version : 5);
	} else if (strcmp(iface, wl_data_device_manager_interface.name) == 0 && pl_manager == NULL) {
		pl_manager_version = version < 3 ? version : 3;
		pl_manager = wl_registry_bind(reg, name, &wl_data_device_manager_interface, pl_manager_version);
	}
}

static void pl_global_remove(void *data, struct wl_registry *reg, uint32_t name) {}

static const struct wl_registry_listener pl_registry_listener = {
	.global = pl_global,
	.global_remove = pl_global_remove,
};

// borealWaylandSetup binds a seat and a data device of Boreal's own on a private queue.
// It returns 0 when drops can be received.
int borealWaylandSetup(uintptr_t display, uintptr_t surface) {
	struct wl_display *d = (struct wl_display *)display;
	pl_surface = (struct wl_surface *)surface;
	pl_queue = wl_display_create_queue(d);

	struct wl_display *wrapped = wl_proxy_create_wrapper(d);
	wl_proxy_set_queue((struct wl_proxy *)wrapped, pl_queue);
	struct wl_registry *reg = wl_display_get_registry(wrapped);
	wl_proxy_wrapper_destroy(wrapped);
	wl_registry_add_listener(reg, &pl_registry_listener, NULL);

	if (wl_display_roundtrip_queue(d, pl_queue) < 0 || pl_seat == NULL || pl_manager == NULL) {
		return -1;
	}

	struct wl_data_device *dev = wl_data_device_manager_get_data_device(pl_manager, pl_seat);
	wl_data_device_add_listener(dev, &pl_dev_listener, NULL);
	wl_display_flush(d);
	return 0;
}

// borealWaylandDispatch blocks handling Boreal's queue; it returns when the connection fails.
void borealWaylandDispatch(uintptr_t display) {
	while (wl_display_dispatch_queue((struct wl_display *)display, pl_queue) >= 0) {
	}
}

// borealWaylandReceive asks for the dropped URI list on fd.
void borealWaylandReceive(uintptr_t display, uintptr_t offer, int fd) {
	wl_data_offer_receive((struct wl_data_offer *)offer, URI_LIST, fd);
	wl_display_flush((struct wl_display *)display);
}

// borealWaylandFinish tells the source the drop is done and frees the offer.
void borealWaylandFinish(uintptr_t display, uintptr_t offer) {
	struct wl_data_offer *o = (struct wl_data_offer *)offer;

	if (pl_manager_version >= 3) {
		wl_data_offer_finish(o);
	}

	pl_destroy_offer(o);
	wl_display_flush((struct wl_display *)display);
}
