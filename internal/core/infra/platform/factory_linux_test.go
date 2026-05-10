//go:build linux

//nolint:testpackage
package platform

import (
	"testing"
)

func TestNewSystemPort_GNOMEWaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if systemPort == nil {
		t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-gnome" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-gnome")
	}
}

func TestNewSystemPort_KDEWaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if systemPort == nil {
		t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-kde" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-kde")
	}
}

func TestNewSystemPort_NoDisplayServerReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if systemPort == nil {
		t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
	}

	if got := systemPort.Capabilities().Platform; got != "linux/unknown" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/unknown")
	}
}

func TestNewSystemPort_SwayWaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "sway")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if systemPort == nil {
		t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-wlroots" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-wlroots")
	}
}
