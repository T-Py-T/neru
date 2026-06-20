//go:build linux

// internal/core/infra/accessibility/element_linux_backend_test.go
// Regression tests pinning how each Linux desktop-environment family resolves
// to Neru's mouse/click/scroll dispatcher in element_linux.go.
// Does not exercise the CGO wlroots/libei backends (covered by VM integration).

//nolint:testpackage // test covers unexported mapping used by the click path
package accessibility

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
)

// TestMapLinuxBackend pins the per-DE dispatcher mapping. wlroots, KDE, and
// GNOME share the same Wayland click/move/scroll path, so all three MUST map to
// linuxBackendWayland. This is the guard that catches a regression where
// extending one DE accidentally drops another out of the Wayland path (the
// exact bug where GNOME mapped to Unknown and every click became a no-op).
func TestMapLinuxBackend(t *testing.T) {
	tests := []struct {
		name     string
		detected platform.LinuxBackend
		want     linuxBackend
	}{
		{"x11 -> x11", platform.BackendX11, linuxBackendX11},
		{"wlroots -> wayland", platform.BackendWaylandWlroots, linuxBackendWayland},
		{"kde -> wayland", platform.BackendWaylandKDE, linuxBackendWayland},
		{"gnome -> wayland", platform.BackendWaylandGNOME, linuxBackendWayland},
		{"other wayland -> unknown", platform.BackendWaylandOther, linuxBackendUnknown},
		{"unknown -> unknown", platform.BackendUnknown, linuxBackendUnknown},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := mapLinuxBackend(testCase.detected)
			if got != testCase.want {
				t.Fatalf("mapLinuxBackend(%v) = %v, want %v",
					testCase.detected, got, testCase.want)
			}
		})
	}
}

// TestWaylandDispatcherFamilyShared asserts the wlroots/KDE/GNOME families
// resolve to the identical dispatcher value. If a change makes them diverge,
// one DE is silently using a different (likely broken) click path.
func TestWaylandDispatcherFamilyShared(t *testing.T) {
	waylandFamily := []platform.LinuxBackend{
		platform.BackendWaylandWlroots,
		platform.BackendWaylandKDE,
		platform.BackendWaylandGNOME,
	}

	want := mapLinuxBackend(platform.BackendWaylandWlroots)
	if want != linuxBackendWayland {
		t.Fatalf("wlroots must resolve to linuxBackendWayland, got %v", want)
	}

	for _, backend := range waylandFamily {
		if got := mapLinuxBackend(backend); got != want {
			t.Fatalf("backend %v resolves to %v, want %v (shared Wayland path)",
				backend, got, want)
		}
	}
}
