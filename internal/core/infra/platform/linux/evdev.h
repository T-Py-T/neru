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

// Absolute uinput pointer, used as the pointer-injection path on compositors
// that do not implement zwlr_virtual_pointer_v1 (notably KWin/KDE). The device
// is created once with the full desktop extent and reused for every move/click.
int neru_uinput_create_pointer(int maxx, int maxy, int *out_fd);
int neru_uinput_move_abs(int fd, int x, int y);
int neru_uinput_button(int fd, int button, int pressed);
void neru_uinput_destroy_pointer(int fd);

#endif /* EVDEV_H */
