//go:build windows

// internal/app/modes/overlay_work_windows.go
// Windows overlay work hook that releases the handler mutex during Win32 UI work.
// Does not implement overlay drawing; callers pass overlay manager operations.

package modes

// runOverlayWork runs fn without holding h.mu when the caller already holds it.
// The keyboard hook also acquires h.mu; blocking Win32 overlay marshaling under
// the same lock stalls mode exit and second grid activation.
func (h *Handler) runOverlayWork(fn func()) {
	if h.mu.TryLock() {
		fn()
		h.mu.Unlock()

		return
	}

	h.mu.Unlock()
	fn()
	h.mu.Lock()
}
