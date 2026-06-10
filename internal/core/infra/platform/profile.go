package platform

import (
	"os"
	"strings"
)

// DisplayServer identifies the active or planned display-system family.
type DisplayServer string

const (
	// DisplayServerCocoa is the native macOS window/display stack.
	DisplayServerCocoa DisplayServer = "cocoa"
	// DisplayServerWayland is the Linux Wayland compositor stack.
	DisplayServerWayland DisplayServer = "wayland"
	// DisplayServerX11 is the Linux X11 stack.
	DisplayServerX11 DisplayServer = "x11"
	// DisplayServerWin32 is the Windows desktop/windowing stack.
	DisplayServerWin32 DisplayServer = "win32"
	// DisplayServerUnknown means the display server could not be identified yet.
	DisplayServerUnknown DisplayServer = "unknown"
)

// BuildMode describes whether a backend is expected to need CGO.
type BuildMode string

const (
	// BuildModePureGo means the backend should build without CGO.
	BuildModePureGo BuildMode = "pure_go"
	// BuildModeCGORequired means the backend depends on CGO/native linkage.
	BuildModeCGORequired BuildMode = "cgo_required"
	// BuildModeBackendDependent means the answer depends on which backend is chosen.
	BuildModeBackendDependent BuildMode = "backend_dependent"
)

const defaultPrimaryModifier = "ctrl"

// BackendPlan describes the intended backend family for one subsystem and
// whether contributors should expect CGO to be required.
type BackendPlan struct {
	Name      string
	BuildMode BuildMode
	Notes     string
}

// Profile describes the platform conventions contributors should target when
// adding OS-specific implementations.
type Profile struct {
	OS              OS
	PrimaryModifier string
	DisplayServer   DisplayServer
	Accessibility   BackendPlan
	Hotkeys         BackendPlan
	KeyboardCapture BackendPlan
	Overlay         BackendPlan
	Notifications   BackendPlan
}

// ProfileFor returns the contributor-facing platform profile for a target OS.
func ProfileFor(target OS) Profile {
	switch target {
	case Darwin:
		return Profile{
			OS:              Darwin,
			PrimaryModifier: "cmd",
			DisplayServer:   DisplayServerCocoa,
			Accessibility: BackendPlan{
				Name:      "axuielement",
				BuildMode: BuildModeCGORequired,
				Notes:     "Objective-C bridge into macOS accessibility APIs",
			},
			Hotkeys: BackendPlan{
				Name:      "carbon-hotkeys",
				BuildMode: BuildModeCGORequired,
				Notes:     "Carbon registration lives behind the Objective-C bridge",
			},
			KeyboardCapture: BackendPlan{
				Name:      "quartz-event-tap",
				BuildMode: BuildModeCGORequired,
				Notes:     "Quartz event taps use CGO-backed Cocoa/CoreGraphics bindings",
			},
			Overlay: BackendPlan{
				Name:      "cocoa-overlay-window",
				BuildMode: BuildModeCGORequired,
				Notes:     "Native overlay windows are implemented through Cocoa",
			},
			Notifications: BackendPlan{
				Name:      "usernotifications/nsalert",
				BuildMode: BuildModeCGORequired,
				Notes:     "Current macOS notifications and alerts use the native bridge",
			},
		}
	case Linux:
		return linuxProfile(DetectLinuxDisplayServer())
	case Windows:
		return Profile{
			OS:              Windows,
			PrimaryModifier: "ctrl",
			DisplayServer:   DisplayServerWin32,
			Accessibility: BackendPlan{
				Name:      "uia",
				BuildMode: BuildModePureGo,
				Notes:     "Prefer COM/Win32 bindings through x/sys or equivalent wrappers",
			},
			Hotkeys: BackendPlan{
				Name:      "RegisterHotKey",
				BuildMode: BuildModePureGo,
				Notes:     "Win32 hotkeys should not require CGO for a first implementation",
			},
			KeyboardCapture: BackendPlan{
				Name:      "low-level keyboard hook",
				BuildMode: BuildModePureGo,
				Notes:     "Hooks are reachable through Win32/syscall bindings",
			},
			Overlay: BackendPlan{
				Name:      "layered win32 window",
				BuildMode: BuildModePureGo,
				Notes:     "Keep the first overlay implementation CGO-free if practical",
			},
			Notifications: BackendPlan{
				Name:      "windows toast",
				BuildMode: BuildModePureGo,
				Notes:     "Toast APIs are expected to be reachable without CGO",
			},
		}
	case Unknown:
		return Profile{
			OS:              Unknown,
			PrimaryModifier: defaultPrimaryModifier,
			DisplayServer:   DisplayServerUnknown,
		}
	default:
		return Profile{
			OS:              Unknown,
			PrimaryModifier: defaultPrimaryModifier,
			DisplayServer:   DisplayServerUnknown,
		}
	}
}

// CurrentProfile returns the contributor-facing profile for the running OS.
func CurrentProfile() Profile {
	return ProfileFor(CurrentOS())
}

func linuxProfile(ds DisplayServer) Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: "ctrl",
		DisplayServer:   ds,
		Accessibility: BackendPlan{
			Name:      "at-spi",
			BuildMode: BuildModePureGo,
			Notes:     "Prefer D-Bus/pure-Go bindings before reaching for CGO",
		},
		Hotkeys: BackendPlan{
			Name:      "x11 or compositor-specific backend",
			BuildMode: BuildModeBackendDependent,
			Notes:     "X11 may stay pure Go; Wayland/compositor paths may need CGO or native helpers",
		},
		KeyboardCapture: BackendPlan{
			Name:      "x11 or compositor-specific backend",
			BuildMode: BuildModeBackendDependent,
			Notes:     "Backend choice determines whether pure Go is enough",
		},
		Overlay: BackendPlan{
			Name:      "x11 window or wayland layer-shell",
			BuildMode: BuildModeBackendDependent,
			Notes:     "Simple X11 overlays may stay pure Go; Wayland paths may require native linkage",
		},
		Notifications: BackendPlan{
			Name:      "freedesktop notifications",
			BuildMode: BuildModePureGo,
			Notes:     "D-Bus notifications should be achievable without CGO",
		},
	}
}

// LinuxProfileForBackend returns the runtime profile for a concrete detected
// Linux backend label (as produced by LinuxBackend.String(), e.g.
// "wayland-kde", "wayland-wlroots", "x11"). Unlike the contributor-facing
// linuxProfile, this reflects the backend Neru actually selected on the running
// system, so `neru doctor` can report what it sees per desktop/compositor
// instead of a generic "backend_dependent" plan. Unknown labels fall back to
// the conservative display-server profile.
func LinuxProfileForBackend(backend string) Profile {
	switch backend {
	case "x11":
		return Profile{
			OS:              Linux,
			PrimaryModifier: defaultPrimaryModifier,
			DisplayServer:   DisplayServerX11,
			Accessibility: BackendPlan{
				Name:      "at-spi",
				BuildMode: BuildModePureGo,
				Notes:     "AT-SPI tree over D-Bus; element geometry via the X11 bridge",
			},
			Hotkeys: BackendPlan{
				Name:      "x11 keygrab",
				BuildMode: BuildModeCGORequired,
				Notes:     "XGrabKey through the X11 C bridge",
			},
			KeyboardCapture: BackendPlan{
				Name:      "x11 event tap",
				BuildMode: BuildModeCGORequired,
				Notes:     "X11 key events through the C bridge",
			},
			Overlay: BackendPlan{
				Name:      "x11 window",
				BuildMode: BuildModeCGORequired,
				Notes:     "Override-redirect X11 window rendered with Cairo",
			},
			Notifications: linuxNotificationsPlan(),
		}
	case "wayland-wlroots", "wayland-kde":
		return waylandLayerShellProfile()
	case "wayland-gnome", "wayland-other":
		return waylandNoOverlayProfile()
	default:
		return linuxProfile(DetectLinuxDisplayServer())
	}
}

// waylandLayerShellProfile describes the backend Neru uses on wlr-layer-shell
// compositors (wlroots and KDE Plasma): evdev for input capture and a
// layer-shell overlay. Injection differs between the two (libei on KDE,
// virtual-pointer on wlroots) but that is not part of the profile surface.
func waylandLayerShellProfile() Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: defaultPrimaryModifier,
		DisplayServer:   DisplayServerWayland,
		Accessibility: BackendPlan{
			Name:      "at-spi",
			BuildMode: BuildModePureGo,
			Notes:     "AT-SPI tree over D-Bus",
		},
		Hotkeys: BackendPlan{
			Name:      "evdev",
			BuildMode: BuildModeCGORequired,
			Notes:     "Global key events read from /dev/input (requires the 'input' group)",
		},
		KeyboardCapture: BackendPlan{
			Name:      "evdev",
			BuildMode: BuildModeCGORequired,
			Notes:     "EVIOCGRAB keyboard capture from /dev/input",
		},
		Overlay: BackendPlan{
			Name:      "wlr-layer-shell",
			BuildMode: BuildModeCGORequired,
			Notes:     "Layer-shell surfaces via the Wayland C client + Cairo",
		},
		Notifications: linuxNotificationsPlan(),
	}
}

// waylandNoOverlayProfile covers Wayland compositors that drive input via evdev
// but do not expose wlr-layer-shell (GNOME/Mutter and other non-wlroots
// compositors), so the overlay is unsupported there.
func waylandNoOverlayProfile() Profile {
	profile := waylandLayerShellProfile()
	profile.Overlay = BackendPlan{
		Name:      "unsupported (no wlr-layer-shell)",
		BuildMode: BuildModeBackendDependent,
		Notes:     "GNOME/Mutter and other non-wlroots compositors do not expose wlr-layer-shell",
	}

	return profile
}

func linuxNotificationsPlan() BackendPlan {
	return BackendPlan{
		Name:      "not implemented",
		BuildMode: BuildModePureGo,
		Notes:     "freedesktop notifications are planned but not wired yet",
	}
}

// DetectLinuxDisplayServer identifies the Linux display stack from environment
// variables. It is intentionally conservative because backend selection is an
// important contributor decision point.
func DetectLinuxDisplayServer() DisplayServer {
	return detectLinuxDisplayServer(
		os.Getenv("XDG_SESSION_TYPE"),
		os.Getenv("WAYLAND_DISPLAY"),
		os.Getenv("DISPLAY"),
	)
}

func detectLinuxDisplayServer(sessionType, waylandDisplay, xDisplay string) DisplayServer {
	switch {
	case strings.EqualFold(sessionType, "wayland"), waylandDisplay != "":
		return DisplayServerWayland
	case strings.EqualFold(sessionType, "x11"), xDisplay != "":
		return DisplayServerX11
	default:
		return DisplayServerUnknown
	}
}
