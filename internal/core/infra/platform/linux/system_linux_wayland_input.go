//go:build linux

package linux

import (
	"image"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// This file is the single routing seam between Neru's Wayland input requests
// and the three non-overlapping injection backends:
//
//   - zwlr_virtual_pointer_v1 / zwp_virtual_keyboard_v1 (wlroots compositors:
//     Sway, Hyprland, niri, River), implemented in the wlroots client.
//   - libei via the org.freedesktop.portal.RemoteDesktop portal (KWin/KDE and
//     GNOME, which do not implement the wlroots input protocols but ship a
//     RemoteDesktop portal backend), implemented in the libei client.
//   - uinput absolute virtual pointer (COSMIC/Smithay, which exposes neither
//     zwlr_virtual_pointer_v1 nor a RemoteDesktop portal), implemented in the
//     uinput slot.
//
// Selection is capability-based, never a hard-coded backend enum: probe
// virtual-pointer first, then the RemoteDesktop portal, else fall back to
// uinput. Screen enumeration and the overlay still go through the wlroots
// client on every family because KWin and cosmic-comp both implement
// zwlr_layer_shell_v1 and zxdg_output_manager_v1. Only input differs, so the
// backend choice lives here rather than inside any client. The cursor position
// is cached in the wlroots client; after a libei or uinput move we mirror the
// new position back into that cache so CursorPosition and screen resolution
// stay correct.

// waylandInputBackend identifies which injection path the running compositor
// requires.
type waylandInputBackend int

const (
	waylandInputWlroots waylandInputBackend = iota
	waylandInputLibei
	waylandInputUinput
)

// waylandResolveInputBackend chooses the input path by capability probe. The
// probes are individually cached (wlroots state, portal presence), so this is
// cheap to call per operation.
func waylandResolveInputBackend() (waylandInputBackend, error) {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return 0, err
	}

	if hasVirtualPointer {
		return waylandInputWlroots, nil
	}

	if remoteDesktopPortalAvailable() {
		return waylandInputLibei, nil
	}

	return waylandInputUinput, nil
}

// libeiModifierKeycodes maps Neru's modifier names to evdev keycodes for the
// libei keyboard path. KWin's RemoteDesktop portal commonly grants only a
// pointer device, so libeiKey may still report these as unsupported.
var libeiModifierKeycodes = map[string]int{
	"shift": 42,  // KEY_LEFTSHIFT
	"ctrl":  29,  // KEY_LEFTCTRL
	"alt":   56,  // KEY_LEFTALT
	"cmd":   125, // KEY_LEFTMETA
}

// WarmWaylandInput pre-establishes the Wayland input backend at daemon startup.
// On a wlroots compositor (or X11/non-Wayland session) it is a cheap no-op. On
// KWin/KDE/GNOME — where input goes through libei via the RemoteDesktop portal —
// it triggers the one-time "Remote Control" consent prompt now, so the first
// user action does not block on the dialog past the IPC timeout. On COSMIC it
// creates the uinput virtual pointer up front (no consent needed). Best-effort:
// errors (no Wayland session, consent declined, /dev/uinput not writable) are
// returned for logging and the lazy path remains as a fallback.
func WarmWaylandInput() error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	switch backend {
	case waylandInputWlroots:
		return nil
	case waylandInputLibei:
		return libeiEnsure()
	case waylandInputUinput:
		return uinputEnsure()
	}

	return nil
}

func waylandMoveCursorToPoint(point image.Point) error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	if backend == waylandInputWlroots {
		return wlrootsMoveCursorToPoint(point)
	}

	if backend == waylandInputUinput {
		if err := uinputMoveAbs(point.X, point.Y); err != nil {
			return err
		}

		return wlrootsSetCursor(point)
	}

	if err := libeiMoveAbs(point.X, point.Y); err != nil {
		return err
	}

	return wlrootsSetCursor(point)
}

func waylandCursorPosition() (image.Point, error) {
	// The cursor cache lives in the wlroots client for both backends; libei
	// moves are mirrored into it by waylandMoveCursorToPoint.
	return wlrootsCursorPosition()
}

func waylandClick(point image.Point, button int) error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	if backend == waylandInputWlroots {
		return wlrootsClick(point, button)
	}

	if err := waylandMoveCursorToPoint(point); err != nil {
		return err
	}

	if backend == waylandInputUinput {
		if err := uinputButton(button, true); err != nil {
			return err
		}

		return uinputButton(button, false)
	}

	if err := libeiButton(button, true); err != nil {
		return err
	}

	return libeiButton(button, false)
}

func waylandButtonEvent(point image.Point, button int, pressed bool) error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	if backend == waylandInputWlroots {
		return wlrootsButtonEvent(point, button, pressed)
	}

	if err := waylandMoveCursorToPoint(point); err != nil {
		return err
	}

	if backend == waylandInputUinput {
		return uinputButton(button, pressed)
	}

	return libeiButton(button, pressed)
}

func waylandButtonRelease(button int) error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	switch backend {
	case waylandInputWlroots:
		return wlrootsButtonRelease(button)
	case waylandInputUinput:
		return uinputButton(button, false)
	case waylandInputLibei:
		return libeiButton(button, false)
	}

	return nil
}

func waylandScroll(axis, delta, discrete int) error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	if backend == waylandInputWlroots {
		return wlrootsScroll(axis, delta, discrete)
	}

	if backend == waylandInputUinput {
		value := discrete
		if value == 0 {
			switch {
			case delta > 0:
				value = 1
			case delta < 0:
				value = -1
			}
		}

		return uinputScroll(axis, value)
	}

	return libeiScroll(axis, delta)
}

func waylandModifierEvent(modifier string, isDown bool) error {
	backend, err := waylandResolveInputBackend()
	if err != nil {
		return err
	}

	// wlroots and COSMIC both expose zwp_virtual_keyboard_manager_v1, so the
	// wlroots client handles modifier injection for both. Only the libei
	// (KDE/GNOME) path uses the portal keyboard device.
	if backend != waylandInputLibei {
		return wlrootsModifierEvent(modifier, isDown)
	}

	keycode, ok := libeiModifierKeycodes[modifier]
	if !ok {
		return derrors.Newf(
			derrors.CodeNotSupported,
			"unsupported modifier %q for libei keyboard injection",
			modifier,
		)
	}

	return libeiKey(keycode, isDown)
}
