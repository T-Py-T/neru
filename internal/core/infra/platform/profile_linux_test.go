//go:build linux

//nolint:testpackage // Same-package tests cover unexported DE profile helpers.
package platform

import (
	"strings"
	"testing"
)

func TestLinuxGNOMEProfile(t *testing.T) {
	got := linuxGNOMEProfile()

	if got.DisplayServer != DisplayServerWaylandGNOME {
		t.Fatalf("DisplayServer = %q, want %q", got.DisplayServer, DisplayServerWaylandGNOME)
	}

	if got.Accessibility.Name == "" || got.Accessibility.BuildMode != "" {
		t.Fatalf("Accessibility = %+v, want user-facing Name only", got.Accessibility)
	}

	if got.Hotkeys.Name == "" {
		t.Fatal("Hotkeys.Name should describe evdev hotkey setup")
	}

	if !strings.Contains(got.KeyboardCapture.Name, "libei") {
		t.Fatalf("KeyboardCapture.Name = %q, want libei portal description", got.KeyboardCapture.Name)
	}

	// GNOME renders overlays through the Shell extension, NOT wlr-layer-shell.
	// This guards against the GNOME profile being copied from / collapsed into
	// the KDE profile.
	if !strings.Contains(got.Overlay.Name, "extension") {
		t.Fatalf("Overlay.Name = %q, want GNOME Shell extension description", got.Overlay.Name)
	}

	if got.Notifications.Name != "not implemented" {
		t.Fatalf("Notifications.Name = %q, want not implemented", got.Notifications.Name)
	}
}

// TestLinuxGNOMEKDEProfilesDistinct guards that extending one DE's profile does
// not regress the other: GNOME and KDE must keep their own display server and
// overlay descriptions.
func TestLinuxGNOMEKDEProfilesDistinct(t *testing.T) {
	gnome := linuxGNOMEProfile()
	kde := linuxKDEProfile()

	if gnome.DisplayServer == kde.DisplayServer {
		t.Fatalf("GNOME and KDE share DisplayServer %q", gnome.DisplayServer)
	}

	if gnome.Overlay.Name == kde.Overlay.Name {
		t.Fatalf("GNOME and KDE share Overlay.Name %q", gnome.Overlay.Name)
	}
}

func TestLinuxKDEProfile(t *testing.T) {
	got := linuxKDEProfile()

	if got.DisplayServer != DisplayServerWaylandKDE {
		t.Fatalf("DisplayServer = %q, want %q", got.DisplayServer, DisplayServerWaylandKDE)
	}

	if got.Accessibility.Name == "" || got.Accessibility.BuildMode != "" {
		t.Fatalf("Accessibility = %+v, want user-facing Name only", got.Accessibility)
	}

	if got.Hotkeys.Name == "" {
		t.Fatal("Hotkeys.Name should describe evdev hotkey setup")
	}

	if got.KeyboardCapture.Name == "" {
		t.Fatal("KeyboardCapture.Name should describe evdev + libei setup")
	}

	if got.Overlay.Name == "" {
		t.Fatal("Overlay.Name should describe wlr-layer-shell via KWin")
	}

	if got.Notifications.Name != "not implemented" {
		t.Fatalf("Notifications.Name = %q, want not implemented", got.Notifications.Name)
	}
}
