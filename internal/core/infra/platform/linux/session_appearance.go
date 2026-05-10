//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/core/ports"

// SessionAppearance probes the host session light/dark preference using the
// same xdg-desktop-portal and kdeglobals sources as SystemAdapter.Capabilities.
// It does not require a supported input backend (X11 / wlroots), so it can run
// on GNOME or KDE Wayland where NewSystemPort would still fail.
func SessionAppearance() (ports.FeatureCapability, bool) {
	value, source, ok := darkModePreference()

	return darkModeCapability(value, source, ok), ok && value == colorSchemeDark
}
