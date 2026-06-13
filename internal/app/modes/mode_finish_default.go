//go:build !windows

// internal/app/modes/mode_finish_default.go
// No-op post-lock hooks for non-Windows navigation mode setup.

package modes

// finishSetNavigationMode runs after h.mu is released following SetMode*.
func (h *Handler) finishSetNavigationMode() {}

// completeModeActivationDefer runs after ActivateModeWithOptions releases h.mu.
func (h *Handler) completeModeActivationDefer(enable bool) {}

// shouldDeferEventTapEnable reports whether event-tap enable waits for unlock.
func (h *Handler) shouldDeferEventTapEnable() bool {
	return false
}
