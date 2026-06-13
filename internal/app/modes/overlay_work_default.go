//go:build !windows

// internal/app/modes/overlay_work_default.go
// No-op overlay work hook for non-Windows platforms.
// Does not implement Win32 UI-thread marshaling or handler mutex release.

package modes

// runOverlayWork runs overlay work while the handler mutex is held on non-Windows
// platforms. Overlay backends marshal to their UI threads internally.
func (h *Handler) runOverlayWork(fn func()) {
	fn()
}
