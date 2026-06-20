//go:build linux

// internal/core/infra/platform/profile_linux.go
// Runtime Linux profile selection for doctor/status output. When KDE Plasma
// Wayland is detected, returns user-facing backend descriptions for that stack.
// It does not perform live capability probes or alter runtime backend selection.

package platform

func linuxProfileForCurrentBackend() Profile {
	switch DetectLinuxBackend() {
	case BackendWaylandKDE:
		return linuxKDEProfile()
	case BackendWaylandCOSMIC:
		return linuxCOSMICProfile()
	case BackendUnknown, BackendX11, BackendWaylandWlroots,
		BackendWaylandGNOME, BackendWaylandOther:
		return linuxProfile(DetectLinuxDisplayServer())
	}

	return linuxProfile(DetectLinuxDisplayServer())
}

func linuxKDEProfile() Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: defaultPrimaryModifier,
		DisplayServer:   DisplayServerWaylandKDE,
		Accessibility: BackendPlan{
			Name: "AT-SPI over D-Bus (hints corrected via KWin geometry bridge)",
		},
		Hotkeys: BackendPlan{
			Name: "evdev from /dev/input (requires input group; bind triggers in KDE System Settings)",
		},
		KeyboardCapture: BackendPlan{
			Name: "evdev capture + libei input via RemoteDesktop portal (consent per daemon launch)",
		},
		Overlay: BackendPlan{
			Name: "wlr-layer-shell via KWin",
		},
		Notifications: BackendPlan{
			Name: "not implemented",
		},
	}
}

func linuxCOSMICProfile() Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: defaultPrimaryModifier,
		DisplayServer:   DisplayServerWaylandCOSMIC,
		Accessibility: BackendPlan{
			Name: "AT-SPI over D-Bus (hints deferred: no COSMIC active-window geometry source yet)",
		},
		Hotkeys: BackendPlan{
			Name: "evdev from /dev/input (requires input group; bind triggers in COSMIC Settings)",
		},
		KeyboardCapture: BackendPlan{
			Name: "evdev capture + uinput virtual pointer (requires /dev/uinput via input group)",
		},
		Overlay: BackendPlan{
			Name: "wlr-layer-shell via cosmic-comp",
		},
		Notifications: BackendPlan{
			Name: "not implemented",
		},
	}
}
