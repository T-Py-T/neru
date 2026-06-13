//go:build !windows

// internal/app/hotkeys_platform_default.go
// Default hotkey registration behavior for non-Windows platforms.

package app

// allowGlobalHotkeysWithoutBundleID reports whether hotkeys register without a bundle ID.
func (a *App) allowGlobalHotkeysWithoutBundleID() bool {
	return false
}
