//go:build !windows

// internal/app/modes/grid_bounds_default.go
// Default grid screen-bounds fallback for non-Windows platforms.

package modes

import "image"

// fallbackGridScreenBounds returns bounds unchanged on macOS and Linux.
func (h *Handler) fallbackGridScreenBounds(bounds image.Rectangle) image.Rectangle {
	return bounds
}
