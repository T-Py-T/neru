//go:build windows

// internal/app/modes/overlay_session_windows.go
// Windows overlay session hook that releases h.mu during Win32 UI marshaling.
// Does not implement drawing; callers pass overlay manager operations.

package modes

// runOverlaySession runs fn without holding h.mu when the caller already holds it.
// The keyboard hook also acquires h.mu; blocking runOnOverlayUI under that lock
// stalls mode exit and grid re-activation.
func (h *Handler) runOverlaySession(fn func()) {
	if h.mu.TryLock() {
		fn()
		h.mu.Unlock()

		return
	}

	h.mu.Unlock()
	fn()
	h.mu.Lock()
}
