//go:build windows

// internal/app/modes/mode_eventtap_windows.go
// Defers navigation event-tap enable until after h.mu is released on Windows.
// Does not start the keyboard hook itself; ActivateModeWithOptions calls enable.

package modes

// enableNavigationEventTapLocked is a no-op on Windows.
// The event tap is enabled after h.mu unlock in ActivateModeWithOptions / SetMode*.
func (h *Handler) enableNavigationEventTapLocked() {}

// enableNavigationEventTap releases the handler lock requirement before
// starting the low-level keyboard hook.
func (h *Handler) enableNavigationEventTap() {
	if h.enableEventTap != nil {
		h.enableEventTap()
	}
}
