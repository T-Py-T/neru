//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: libei-1.0 liboeffis-1.0
#include <stdlib.h>
#include "libei_client.h"
*/
import "C"

import (
	"sync"
	"sync/atomic"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// This file is the KDE Plasma Wayland input slot (compositor sub-slot "kde",
// sibling to the "wlroots" slot). KWin does not implement
// zwlr_virtual_pointer_v1, so input is injected through libei via the
// org.freedesktop.portal.RemoteDesktop portal. The libei mechanism itself
// (libei_client.c) is DE-agnostic; if another compositor (e.g. GNOME) later
// routes input through libei, factor the shared pieces out rather than
// duplicating them here. Runtime selection happens in
// system_linux_wayland_input.go via the LinuxBackend family.

// libeiWarmupTimeoutMs bounds a single consent wait. It is long because the
// consent dialog must stay on screen long enough for the user to find and
// approve the one-time "Remote Control" prompt. Startup warm-up and the
// on-demand recovery warm-up both use it: once approved, every later action
// reuses the session with no further wait or prompt.
const libeiWarmupTimeoutMs = 120000

// libeiState owns the libei/RemoteDesktop session used for input injection on
// compositors without zwlr_virtual_pointer_v1 (KWin/KDE, GNOME/Mutter). The
// session is established lazily so read-only probes (screen bounds, `neru
// doctor`) never trigger the portal consent prompt.
type libeiState struct {
	mu     sync.Mutex
	client *C.NeruEiClient
	ready  bool

	// warming guards a single in-flight background consent wait so concurrent
	// input ops never stack multiple RemoteDesktop dialogs.
	warming atomic.Bool
}

var globalLibeiState = &libeiState{}

// ensureLockedTimeout establishes the portal session with an explicit connect
// timeout. The caller holds mu.
func (s *libeiState) ensureLockedTimeout(timeoutMs int) error {
	if s.ready {
		return nil
	}

	client := C.neru_ei_connect(C.int(timeoutMs))
	if client == nil {
		return derrors.New(
			derrors.CodeActionFailed,
			"could not establish a libei input session via the RemoteDesktop "+
				"portal; approve the one-time \"Remote Control\" consent prompt "+
				"(KDE Plasma and GNOME route input through xdg-desktop-portal "+
				"because they do not implement zwlr_virtual_pointer_v1)",
		)
	}

	s.client = client
	s.ready = true

	return nil
}

// libeiEnsure establishes the portal session without injecting input. The
// daemon calls this at startup (via WarmWaylandInput) so the one-time consent
// prompt is handled before any action, instead of blocking the first action
// past the IPC timeout. This holds mu across the long consent wait; mid-action
// input uses tryAcquire so it never blocks here.
func libeiEnsure() error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	return globalLibeiState.ensureLockedTimeout(libeiWarmupTimeoutMs)
}

// kickWarm starts at most one background consent wait. It is the on-demand
// recovery path for when startup warm-up was missed or declined: a mid-action
// input call that finds no session calls this, which brings up a SINGLE, stable
// RemoteDesktop consent dialog (held for libeiWarmupTimeoutMs) the user can
// actually approve. This replaces the old behavior where every action ran a
// short inline connect that created then immediately tore down the dialog on a
// ~1.5s timeout, flickering it faster than anyone could click "Share". The
// goroutine holds mu for the whole wait; concurrent input ops see TryLock fail
// and fail fast rather than freezing the eventtap goroutine or stacking dialogs.
func (s *libeiState) kickWarm() {
	if !s.warming.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer s.warming.Store(false)

		s.mu.Lock()
		defer s.mu.Unlock()

		_ = s.ensureLockedTimeout(libeiWarmupTimeoutMs)
	}()
}

// tryAcquire grabs mu without blocking. When the session is already established
// it leaves the caller owning mu (which it MUST Unlock). When the session is
// not ready it never connects inline: it kicks a single background warm-up (one
// stable consent dialog) and returns a fast, non-blocking error so the action
// releases the keyboard grab immediately. This keeps the GNOME/KDE
// RemoteDesktop prompt approvable mid-session without flickering it.
func (s *libeiState) tryAcquire() error {
	if !s.mu.TryLock() {
		return derrors.New(
			derrors.CodeActionFailed,
			"libei input session busy (RemoteDesktop consent prompt is showing); "+
				"approve the one-time \"Remote Control\" prompt, then retry",
		)
	}

	if s.ready {
		return nil
	}

	s.mu.Unlock()
	s.kickWarm()

	return derrors.New(
		derrors.CodeActionFailed,
		"libei input session not ready; approve the one-time \"Remote Control\" "+
			"consent prompt now showing, then retry the action",
	)
}

func libeiMoveAbs(x, y int) error {
	if err := globalLibeiState.tryAcquire(); err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	if C.neru_ei_move_abs(globalLibeiState.client, C.int(x), C.int(y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"libei failed to move pointer to (%d, %d)",
			x, y,
		)
	}

	return nil
}

func libeiButton(button int, pressed bool) error {
	if err := globalLibeiState.tryAcquire(); err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_ei_button(globalLibeiState.client, C.int(button), pressedInt) == 0 {
		return derrors.New(derrors.CodeActionFailed, "libei failed to emit button event")
	}

	return nil
}

func libeiScroll(axis, delta int) error {
	if err := globalLibeiState.tryAcquire(); err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	if C.neru_ei_scroll(globalLibeiState.client, C.int(axis), C.int(delta)) == 0 {
		return derrors.New(derrors.CodeActionFailed, "libei failed to emit scroll event")
	}

	return nil
}

func libeiKey(keycode int, pressed bool) error {
	if err := globalLibeiState.tryAcquire(); err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_ei_key(globalLibeiState.client, C.int(keycode), pressedInt) == 0 {
		return derrors.New(
			derrors.CodeNotSupported,
			"libei keyboard injection unavailable; the RemoteDesktop portal "+
				"session did not grant a keyboard device",
		)
	}

	return nil
}
