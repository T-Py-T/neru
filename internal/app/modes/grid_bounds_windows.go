//go:build windows

// internal/app/modes/grid_bounds_windows.go
// Windows grid screen-bounds fallback via the overlay manager backend.

package modes

import (
	"image"

	"go.uber.org/zap"
)

// fallbackGridScreenBounds uses overlay window bounds when system.ScreenBounds fails.
func (h *Handler) fallbackGridScreenBounds(bounds image.Rectangle) image.Rectangle {
	if bounds.Dx() > 0 && bounds.Dy() > 0 {
		return bounds
	}

	type overlayBoundsProvider interface {
		ActiveScreenBounds() (image.Rectangle, bool)
	}

	provider, ok := h.overlayManager.(overlayBoundsProvider)
	if !ok {
		return bounds
	}

	fallback, ok := provider.ActiveScreenBounds()
	if !ok || fallback.Dx() == 0 || fallback.Dy() == 0 {
		return bounds
	}

	h.logger.Warn(
		"Using overlay window bounds as grid screen bounds fallback",
		zap.Int("width", fallback.Dx()),
		zap.Int("height", fallback.Dy()),
	)

	return fallback
}
