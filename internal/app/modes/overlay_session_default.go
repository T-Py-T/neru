//go:build !windows

// internal/app/modes/overlay_session_default.go
// Default overlay session hook for non-Windows platforms.
// Does not release the handler mutex; overlay managers marshal internally.

package modes

// runOverlaySession runs overlay manager work while the handler mutex is held.
func (h *Handler) runOverlaySession(fn func()) {
	fn()
}
