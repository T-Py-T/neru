//go:build !windows

// internal/app/modes/mode_eventtap_default.go
// Enables the navigation event tap while h.mu is held on non-Windows platforms.

package modes

// enableNavigationEventTapLocked turns on keyboard capture for the active mode.
// Caller must hold h.mu.
func (h *Handler) enableNavigationEventTapLocked() {
	if h.enableEventTap != nil {
		h.enableEventTap()
	}
}
