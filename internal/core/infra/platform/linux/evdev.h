#ifndef EVDEV_H
#define EVDEV_H

#include <linux/input.h>
#include <stddef.h>
#include <sys/types.h>

int neru_evdev_grab(int fd, int grab);
int neru_evdev_key_down(int fd, unsigned int keycode);
int neru_evdev_is_keyboard(int fd);
int neru_evdev_get_name(int fd, char *name, size_t name_size);
int neru_evdev_get_bustype(int fd);
ssize_t neru_evdev_read_event(int fd, struct input_event *event);
int neru_uinput_create_scroll(int *out_fd);
int neru_uinput_scroll(int fd, int axis, int value);

/* Absolute virtual pointer (move + buttons), used by compositors that expose
 * neither zwlr_virtual_pointer_v1 nor a RemoteDesktop portal (e.g. COSMIC).
 * max_x/max_y are the absolute axis maxima (screen size in pixels). */
int neru_uinput_create_pointer(int *out_fd, int max_x, int max_y);
int neru_uinput_move_abs(int fd, int x, int y);
int neru_uinput_button(int fd, int button, int pressed);

#endif /* EVDEV_H */
