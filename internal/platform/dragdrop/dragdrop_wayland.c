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
static struct wl_display *pl_display;
static struct wl_data_device *pl_device;
static struct wl_pointer *pl_pointer;
static uint32_t pl_press_serial;
static int pl_pressed;
static int pl_dragging;

static void pl_offer_offer(void *data, struct wl_data_offer *offer, const char *mime) {
	struct pl_offer *state = data;

	if (strcmp(mime, URI_LIST) == 0) {
		state->has_uris = 1;
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

static void pl_dev_data_offer(void *data, struct wl_data_device *device, struct wl_data_offer *offer) {
	struct pl_offer *state = calloc(1, sizeof(*state));
	wl_data_offer_add_listener(offer, &pl_offer_listener, state);
}

static void pl_dev_enter(void *data, struct wl_data_device *device, uint32_t serial, struct wl_surface *surface, wl_fixed_t x, wl_fixed_t y, struct wl_data_offer *offer) {
	if (pl_current != NULL && pl_current != offer) {
		pl_destroy_offer(pl_current);
	}

	pl_current = offer;

	if (offer == NULL) {
		return;
	}

	struct pl_offer *state = wl_data_offer_get_user_data(offer);

	if (pl_dragging || surface != pl_surface || state == NULL || !state->has_uris) {
		wl_data_offer_accept(offer, serial, NULL);
		return;
	}

	wl_data_offer_accept(offer, serial, URI_LIST);

	if (pl_manager_version >= 3) {
		wl_data_offer_set_actions(offer, WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY, WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY);
	}

	borealDragHover(1);
}

static void pl_dev_leave(void *data, struct wl_data_device *device) {
	borealDragHover(0);

	if (pl_current != NULL) {
		pl_destroy_offer(pl_current);
		pl_current = NULL;
	}
}

static void pl_dev_motion(void *data, struct wl_data_device *device, uint32_t t, wl_fixed_t x, wl_fixed_t y) {}

static void pl_dev_drop(void *data, struct wl_data_device *device) {
	struct wl_data_offer *offer = pl_current;
	pl_current = NULL;
	borealDragHover(0);

	if (offer == NULL) {
		return;
	}

	struct pl_offer *state = wl_data_offer_get_user_data(offer);

	if (state == NULL || !state->has_uris) {
		pl_destroy_offer(offer);
		return;
	}

	borealWaylandDrop((uintptr_t)offer);
}

static void pl_dev_selection(void *data, struct wl_data_device *device, struct wl_data_offer *offer) {
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

static void pl_pointer_enter(void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface, wl_fixed_t x, wl_fixed_t y) {}

static void pl_pointer_leave(void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface) {
	pl_pressed = 0;
}

static void pl_pointer_motion(void *data, struct wl_pointer *pointer, uint32_t time, wl_fixed_t x, wl_fixed_t y) {}

static void pl_pointer_button(void *data, struct wl_pointer *pointer, uint32_t serial, uint32_t time, uint32_t button, uint32_t state) {
	pl_pressed = state == WL_POINTER_BUTTON_STATE_PRESSED;

	if (pl_pressed) {
		pl_press_serial = serial;
	}
}

static void pl_pointer_axis(void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis, wl_fixed_t value) {}

static void pl_pointer_frame(void *data, struct wl_pointer *pointer) {}

static void pl_pointer_axis_source(void *data, struct wl_pointer *pointer, uint32_t source) {}

static void pl_pointer_axis_stop(void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis) {}

static void pl_pointer_axis_discrete(void *data, struct wl_pointer *pointer, uint32_t axis, int32_t discrete) {}

static const struct wl_pointer_listener pl_pointer_listener = {
	.enter = pl_pointer_enter,
	.leave = pl_pointer_leave,
	.motion = pl_pointer_motion,
	.button = pl_pointer_button,
	.axis = pl_pointer_axis,
	.frame = pl_pointer_frame,
	.axis_source = pl_pointer_axis_source,
	.axis_stop = pl_pointer_axis_stop,
	.axis_discrete = pl_pointer_axis_discrete,
};

static void pl_source_target(void *data, struct wl_data_source *source, const char *mime) {}

static void pl_source_send(void *data, struct wl_data_source *source, const char *mime, int32_t fd) {
	borealWaylandSend(fd);
}

static void pl_source_done(struct wl_data_source *source) {
	pl_dragging = 0;
	wl_data_source_destroy(source);
}

static void pl_source_cancelled(void *data, struct wl_data_source *source) {
	pl_source_done(source);
}

static void pl_source_dropped(void *data, struct wl_data_source *source) {}

static void pl_source_finished(void *data, struct wl_data_source *source) {
	pl_source_done(source);
}

static void pl_source_action(void *data, struct wl_data_source *source, uint32_t action) {}

static const struct wl_data_source_listener pl_source_listener = {
	.target = pl_source_target,
	.send = pl_source_send,
	.cancelled = pl_source_cancelled,
	.dnd_drop_performed = pl_source_dropped,
	.dnd_finished = pl_source_finished,
	.action = pl_source_action,
};

static void pl_global(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	if (strcmp(interface, wl_seat_interface.name) == 0 && pl_seat == NULL) {
		pl_seat = wl_registry_bind(registry, name, &wl_seat_interface, version < 5 ? version : 5);
	} else if (strcmp(interface, wl_data_device_manager_interface.name) == 0 && pl_manager == NULL) {
		pl_manager_version = version < 3 ? version : 3;
		pl_manager = wl_registry_bind(registry, name, &wl_data_device_manager_interface, pl_manager_version);
	}
}

static void pl_global_remove(void *data, struct wl_registry *registry, uint32_t name) {}

static const struct wl_registry_listener pl_registry_listener = {
	.global = pl_global,
	.global_remove = pl_global_remove,
};

int borealWaylandSetup(uintptr_t handle, uintptr_t surface) {
	struct wl_display *display = (struct wl_display *)handle;
	pl_surface = (struct wl_surface *)surface;
	pl_queue = wl_display_create_queue(display);

	struct wl_display *wrapped = wl_proxy_create_wrapper(display);
	wl_proxy_set_queue((struct wl_proxy *)wrapped, pl_queue);
	struct wl_registry *registry = wl_display_get_registry(wrapped);
	wl_proxy_wrapper_destroy(wrapped);
	wl_registry_add_listener(registry, &pl_registry_listener, NULL);

	if (wl_display_roundtrip_queue(display, pl_queue) < 0 || pl_seat == NULL || pl_manager == NULL) {
		return -1;
	}

	pl_display = display;
	pl_device = wl_data_device_manager_get_data_device(pl_manager, pl_seat);
	wl_data_device_add_listener(pl_device, &pl_dev_listener, NULL);
	pl_pointer = wl_seat_get_pointer(pl_seat);
	wl_pointer_add_listener(pl_pointer, &pl_pointer_listener, NULL);
	wl_display_flush(display);
	return 0;
}

int borealWaylandStartDrag(void) {
	if (pl_device == NULL || !pl_pressed) {
		return -1;
	}

	struct wl_data_source *source = wl_data_device_manager_create_data_source(pl_manager);
	wl_data_source_offer(source, URI_LIST);

	if (pl_manager_version >= 3) {
		wl_data_source_set_actions(source, WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY);
	}

	wl_data_source_add_listener(source, &pl_source_listener, NULL);
	pl_dragging = 1;
	wl_data_device_start_drag(pl_device, source, pl_surface, NULL, pl_press_serial);
	wl_display_flush(pl_display);
	return 0;
}

void borealWaylandDispatch(uintptr_t display) {
	while (wl_display_dispatch_queue((struct wl_display *)display, pl_queue) >= 0) {
	}
}

void borealWaylandReceive(uintptr_t display, uintptr_t offer, int fd) {
	wl_data_offer_receive((struct wl_data_offer *)offer, URI_LIST, fd);
	wl_display_flush((struct wl_display *)display);
}

void borealWaylandFinish(uintptr_t display, uintptr_t offer) {
	struct wl_data_offer *state = (struct wl_data_offer *)offer;

	if (pl_manager_version >= 3) {
		wl_data_offer_finish(state);
	}

	pl_destroy_offer(state);
	wl_display_flush((struct wl_display *)display);
}
