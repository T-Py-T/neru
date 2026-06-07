#include "evdev.h"

#include <errno.h>
#include <fcntl.h>
#include <linux/input.h>
#include <linux/uinput.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

int neru_evdev_grab(int fd, int grab) { return ioctl(fd, EVIOCGRAB, grab); }

int neru_evdev_key_down(int fd, unsigned int keycode) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGKEY(sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}

	return (key_bits[keycode / (8 * sizeof(unsigned long))] >> (keycode % (8 * sizeof(unsigned long)))) & 1UL;
}

int neru_evdev_is_keyboard(int fd) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}

#define NERU_TEST_KEY(bits, key)                                                                                       \
	((bits[(key) / (8 * sizeof(unsigned long))] >> ((key) % (8 * sizeof(unsigned long)))) & 1UL)

	return NERU_TEST_KEY(key_bits, KEY_Q) && NERU_TEST_KEY(key_bits, KEY_W) && NERU_TEST_KEY(key_bits, KEY_E) &&
	       NERU_TEST_KEY(key_bits, KEY_R) && NERU_TEST_KEY(key_bits, KEY_SPACE) && NERU_TEST_KEY(key_bits, KEY_ENTER);
}

int neru_evdev_get_name(int fd, char *name, size_t name_size) {
	int r = ioctl(fd, EVIOCGNAME(name_size), name);
	if (r < 0)
		return -1;
	return r;
}

int neru_evdev_get_bustype(int fd) {
	struct input_id id;
	if (ioctl(fd, EVIOCGID, &id) < 0) {
		return -1;
	}
	return id.bustype;
}

ssize_t neru_evdev_read_event(int fd, struct input_event *event) { return read(fd, event, sizeof(struct input_event)); }

int neru_uinput_create_scroll(int *out_fd) {
	int fd = open("/dev/uinput", O_RDWR);
	if (fd < 0) {
		fd = open("/dev/input/uinput", O_RDWR);
	}
	if (fd < 0) {
		return 0;
	}

	if (ioctl(fd, UI_SET_EVBIT, EV_REL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_WHEEL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_HWHEEL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_WHEEL_HI_RES) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_HWHEEL_HI_RES) < 0) {
		close(fd);
		return 0;
	}

	struct uinput_setup usetup;
	memset(&usetup, 0, sizeof(usetup));
	usetup.id.bustype = BUS_USB;
	usetup.id.vendor = 0x1234;
	usetup.id.product = 0x5678;
	strcpy(usetup.name, "neru-scroll");
	if (ioctl(fd, UI_DEV_SETUP, &usetup) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_DEV_CREATE) < 0) {
		close(fd);
		return 0;
	}

	*out_fd = fd;
	return 1;
}

int neru_uinput_scroll(int fd, int axis, int value) {
	struct input_event ev;
	memset(&ev, 0, sizeof(ev));

	ev.type = EV_REL;
	ev.code = (axis == 0) ? REL_WHEEL_HI_RES : REL_HWHEEL_HI_RES;
	ev.value = value * 120;
	ssize_t w1 = write(fd, &ev, sizeof(ev));

	memset(&ev, 0, sizeof(ev));
	ev.type = EV_REL;
	ev.code = (axis == 0) ? REL_WHEEL : REL_HWHEEL;
	ev.value = value;
	ssize_t w2 = write(fd, &ev, sizeof(ev));

	memset(&ev, 0, sizeof(ev));
	ev.type = EV_SYN;
	ev.code = SYN_REPORT;
	ev.value = 0;
	ssize_t w3 = write(fd, &ev, sizeof(ev));

	return (w1 == sizeof(ev) && w2 == sizeof(ev) && w3 == sizeof(ev)) ? 1 : 0;
}

int neru_uinput_create_pointer(int maxx, int maxy, int *out_fd) {
	int fd = open("/dev/uinput", O_RDWR);
	if (fd < 0) {
		fd = open("/dev/input/uinput", O_RDWR);
	}
	if (fd < 0) {
		return 0;
	}

	if (ioctl(fd, UI_SET_EVBIT, EV_KEY) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_KEYBIT, BTN_LEFT) < 0 || ioctl(fd, UI_SET_KEYBIT, BTN_RIGHT) < 0 ||
	    ioctl(fd, UI_SET_KEYBIT, BTN_MIDDLE) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_EVBIT, EV_ABS) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_ABSBIT, ABS_X) < 0 || ioctl(fd, UI_SET_ABSBIT, ABS_Y) < 0) {
		close(fd);
		return 0;
	}

	// Map the device's absolute range 1:1 onto the desktop extent so the Go
	// side can pass compositor coordinates directly.
	struct uinput_abs_setup abs_x;
	struct uinput_abs_setup abs_y;
	memset(&abs_x, 0, sizeof(abs_x));
	memset(&abs_y, 0, sizeof(abs_y));
	abs_x.code = ABS_X;
	abs_x.absinfo.minimum = 0;
	abs_x.absinfo.maximum = maxx;
	abs_y.code = ABS_Y;
	abs_y.absinfo.minimum = 0;
	abs_y.absinfo.maximum = maxy;
	if (ioctl(fd, UI_ABS_SETUP, &abs_x) < 0 || ioctl(fd, UI_ABS_SETUP, &abs_y) < 0) {
		close(fd);
		return 0;
	}

	struct uinput_setup usetup;
	memset(&usetup, 0, sizeof(usetup));
	usetup.id.bustype = BUS_USB;
	usetup.id.vendor = 0x1234;
	usetup.id.product = 0x5679;
	strcpy(usetup.name, "neru-pointer");
	if (ioctl(fd, UI_DEV_SETUP, &usetup) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_DEV_CREATE) < 0) {
		close(fd);
		return 0;
	}

	*out_fd = fd;
	return 1;
}

static int neru_uinput_emit(int fd, int type, int code, int value) {
	struct input_event ev;
	memset(&ev, 0, sizeof(ev));
	ev.type = (unsigned short)type;
	ev.code = (unsigned short)code;
	ev.value = value;
	return write(fd, &ev, sizeof(ev)) == (ssize_t)sizeof(ev);
}

int neru_uinput_move_abs(int fd, int x, int y) {
	int ok = 1;
	ok &= neru_uinput_emit(fd, EV_ABS, ABS_X, x);
	ok &= neru_uinput_emit(fd, EV_ABS, ABS_Y, y);
	ok &= neru_uinput_emit(fd, EV_SYN, SYN_REPORT, 0);
	return ok;
}

int neru_uinput_button(int fd, int button, int pressed) {
	int ok = 1;
	ok &= neru_uinput_emit(fd, EV_KEY, button, pressed ? 1 : 0);
	ok &= neru_uinput_emit(fd, EV_SYN, SYN_REPORT, 0);
	return ok;
}

void neru_uinput_destroy_pointer(int fd) {
	if (fd >= 0) {
		ioctl(fd, UI_DEV_DESTROY);
		close(fd);
	}
}
