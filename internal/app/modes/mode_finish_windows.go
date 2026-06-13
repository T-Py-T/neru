//go:build windows

// internal/app/modes/mode_finish_windows.go
// Post-lock hooks that start the keyboard hook after h.mu is released.

package modes

// finishSetNavigationMode enables the event tap after SetMode* releases h.mu.
func (h *Handler) finishSetNavigationMode() {
	h.enableNavigationEventTap()
}

// completeModeActivationDefer enables the event tap after mode activation.
func (h *Handler) completeModeActivationDefer(enable bool) {
	if enable {
		h.enableNavigationEventTap()
	}
}

// shouldDeferEventTapEnable reports whether event-tap enable waits for unlock.
func (h *Handler) shouldDeferEventTapEnable() bool {
	return true
}
