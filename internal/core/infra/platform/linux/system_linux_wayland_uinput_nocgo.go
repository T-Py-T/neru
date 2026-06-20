//go:build linux && !cgo

package linux

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// COSMIC Wayland input slot (non-CGO stub). uinput virtual-pointer injection
// requires CGO; these stubs keep the Wayland input dispatcher buildable in the
// (unsupported) non-CGO configuration.

func uinputEnsure() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"uinput backend requires CGO-enabled Linux builds",
	)
}

func uinputMoveAbs(x, y int) error {
	_, _ = x, y

	return derrors.New(
		derrors.CodeNotSupported,
		"uinput backend requires CGO-enabled Linux builds",
	)
}

func uinputButton(button int, pressed bool) error {
	_, _ = button, pressed

	return derrors.New(
		derrors.CodeNotSupported,
		"uinput backend requires CGO-enabled Linux builds",
	)
}

func uinputScroll(axis, value int) error {
	_, _ = axis, value

	return derrors.New(
		derrors.CodeNotSupported,
		"uinput backend requires CGO-enabled Linux builds",
	)
}
