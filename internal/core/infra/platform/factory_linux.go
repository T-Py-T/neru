//go:build linux

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns a Linux SystemPort implementation for every detected
// backend. Input-heavy features may still return CodeNotSupported at call
// time on GNOME/KDE/unknown, but the adapter is always constructible so
// theme/capability probes and other read-only paths work consistently.
func NewSystemPort() (ports.SystemPort, error) {
	return linux.NewSystemAdapter(detectLinuxBackend().String()), nil
}

// ShowConfigOnboardingAlert is a stub on Linux.
func ShowConfigOnboardingAlert(_ string) int {
	return ConfigOnboardingDefaults
}

// ShowConfigValidationErrorAlert is a stub on Linux.
func ShowConfigValidationErrorAlert(_, _ string) int {
	return ConfigValidationOK
}

// CheckAccessibilityPermissions is always true on Linux for startup gating.
func CheckAccessibilityPermissions() bool {
	return true
}

// ShowAccessibilityPermissionStartupAlert is a no-op on Linux.
func ShowAccessibilityPermissionStartupAlert() int {
	return AccessibilityPermissionStartupGranted
}
