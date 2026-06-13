//go:build windows

// internal/app/hotkeys_platform_windows.go
// Windows hotkey registration when UIA cannot resolve a focused app bundle ID.

package app

// allowGlobalHotkeysWithoutBundleID reports whether hotkeys register without a bundle ID.
func (a *App) allowGlobalHotkeysWithoutBundleID() bool {
	return true
}
