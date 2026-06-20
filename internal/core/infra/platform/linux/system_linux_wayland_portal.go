//go:build linux

package linux

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

// RemoteDesktop portal probe. The Wayland input dispatcher uses it to tell the
// libei backend (KDE, GNOME: org.freedesktop.portal.RemoteDesktop present) apart
// from the uinput backend (COSMIC: xdg-desktop-portal-cosmic implements only
// Access/FileChooser/Screenshot/Settings/ScreenCast, so RemoteDesktop is
// absent). The result is fixed for the session, so it is probed once.

const (
	portalBusName          = "org.freedesktop.portal.Desktop"
	portalObjectPath       = "/org/freedesktop/portal/desktop"
	remoteDesktopInterface = "org.freedesktop.portal.RemoteDesktop"
)

var (
	remoteDesktopPortalOnce    sync.Once
	remoteDesktopPortalPresent bool
)

// remoteDesktopPortalAvailable reports whether the active xdg-desktop-portal
// backend implements org.freedesktop.portal.RemoteDesktop (required for the
// libei input path). Cached after the first call.
func remoteDesktopPortalAvailable() bool {
	remoteDesktopPortalOnce.Do(func() {
		remoteDesktopPortalPresent = probeRemoteDesktopPortal()
	})

	return remoteDesktopPortalPresent
}

func probeRemoteDesktopPortal() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()

	// Reading the RemoteDesktop "version" property succeeds only when the
	// portal frontend actually exposes that interface; on COSMIC the call
	// fails with an unknown-interface error.
	obj := conn.Object(portalBusName, dbus.ObjectPath(portalObjectPath))

	var version dbus.Variant

	callErr := obj.Call(
		"org.freedesktop.DBus.Properties.Get", 0,
		remoteDesktopInterface, "version",
	).Store(&version)

	return callErr == nil
}
