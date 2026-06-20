//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: wayland-client xkbcommon
#include <stdlib.h>
#include "evdev.h"
*/
import "C"

import (
	"sync"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// This file is the COSMIC (Smithay) Wayland input slot, sibling to the
// "wlroots" and "kde" slots. cosmic-comp exposes neither
// zwlr_virtual_pointer_v1 nor a RemoteDesktop portal (xdg-desktop-portal-cosmic
// implements only Access/FileChooser/Screenshot/Settings/ScreenCast), so the
// wlroots and libei paths are both unavailable. Input is injected through a
// kernel uinput virtual pointer instead: libinput maps an absolute ABS_X/ABS_Y
// device 1:1 onto the screen, so moves land at the exact pixel. The overlay and
// screen geometry still come from the shared wlroots client (cosmic-comp does
// expose zwlr_layer_shell_v1 + zxdg_output_manager_v1). Runtime selection
// happens in system_linux_wayland_input.go.
//
// Host requirement: write access to /dev/uinput. The clean way is a udev rule
// granting the `input` group, mirroring the evdev hotkey requirement:
//   KERNEL=="uinput", GROUP="input", MODE="0660"
// (see docs/LINUX_SETUP.md). This needs no portal and no consent prompt.

// uinputState owns the uinput virtual pointer device. Unlike libei there is no
// portal handshake or consent prompt, so creation is cheap and never blocks on
// user interaction; a plain mutex around the fast ioctl/write path is enough.
type uinputState struct {
	mu    sync.Mutex
	fd    C.int
	ready bool
}

var globalUinputState = &uinputState{}

// ensureLocked creates the virtual pointer device on first use, sized to the
// current screen bounds so absolute coordinates map 1:1 to pixels. The caller
// holds mu.
func (s *uinputState) ensureLocked() error {
	if s.ready {
		return nil
	}

	bounds, err := wlrootsMaxBounds()
	if err != nil {
		return err
	}

	maxX := bounds.Dx() - 1
	maxY := bounds.Dy() - 1
	if maxX <= 0 {
		maxX = wlrootsDefaultWidth - 1
	}
	if maxY <= 0 {
		maxY = wlrootsDefaultHeight - 1
	}

	var fd C.int
	if C.neru_uinput_create_pointer(&fd, C.int(maxX), C.int(maxY)) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"could not create the uinput virtual pointer; ensure /dev/uinput is "+
				"writable (add a udev rule granting the `input` group: "+
				"KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\")",
		)
	}

	s.fd = fd
	s.ready = true

	return nil
}

func (s *uinputState) ensure() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ensureLocked()
}

// uinputEnsure creates the virtual pointer ahead of the first action. The
// daemon calls this at startup (via WarmWaylandInput) so device creation and
// the brief libinput hotplug settle happen before the user's first move.
func uinputEnsure() error {
	return globalUinputState.ensure()
}

func uinputMoveAbs(x, y int) error {
	if err := globalUinputState.ensure(); err != nil {
		return err
	}

	globalUinputState.mu.Lock()
	defer globalUinputState.mu.Unlock()

	if C.neru_uinput_move_abs(globalUinputState.fd, C.int(x), C.int(y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"uinput failed to move pointer to (%d, %d)",
			x, y,
		)
	}

	return nil
}

func uinputButton(button int, pressed bool) error {
	if err := globalUinputState.ensure(); err != nil {
		return err
	}

	globalUinputState.mu.Lock()
	defer globalUinputState.mu.Unlock()

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_uinput_button(globalUinputState.fd, C.int(button), pressedInt) == 0 {
		return derrors.New(derrors.CodeActionFailed, "uinput failed to emit button event")
	}

	return nil
}

func uinputScroll(axis, value int) error {
	if err := globalUinputState.ensure(); err != nil {
		return err
	}

	globalUinputState.mu.Lock()
	defer globalUinputState.mu.Unlock()

	if C.neru_uinput_scroll(globalUinputState.fd, C.int(axis), C.int(value)) == 0 {
		return derrors.New(derrors.CodeActionFailed, "uinput failed to emit scroll event")
	}

	return nil
}
