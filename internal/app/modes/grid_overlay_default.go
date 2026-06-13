//go:build !windows

// internal/app/modes/grid_overlay_default.go
// Default grid overlay presentation for macOS and Linux.
// Does not implement platform drawing; delegates to overlay.Manager.

package modes

import (
	"go.uber.org/zap"

	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

// presentGridOverlay clears prior content, draws the grid, resizes, and shows.
// Matches upstream/main activation order.
func (h *Handler) presentGridOverlay(gridInstance *domainGrid.Grid) error {
	h.overlayManager.Clear()

	drawErr := h.renderer.DrawGrid(gridInstance, "")
	if drawErr != nil {
		return drawErr
	}

	h.overlayManager.ResizeToActiveScreen()
	h.overlayManager.Show()

	return nil
}

// refreshGridOverlayInput redraws grid match state during navigation.
func (h *Handler) refreshGridOverlayInput(
	gridInstance *domainGrid.Grid,
	input string,
	forceRedraw bool,
) {
	if forceRedraw {
		h.overlayManager.Clear()

		gridErr := h.renderer.DrawGrid(gridInstance, input)
		if gridErr != nil {
			h.logger.Error("Failed to redraw grid", zap.Error(gridErr))

			return
		}

		h.overlayManager.Show()
	}

	hideUnmatched := h.config.Grid.HideUnmatched && len(input) > 0
	h.renderer.SetHideUnmatched(hideUnmatched)
	h.renderer.UpdateGridMatches(input)
}

// clearAndHideOverlaySurface clears and hides the overlay manager.
func (h *Handler) clearAndHideOverlaySurface() {
	h.overlayManager.Clear()
	h.overlayManager.Hide()
}

// resizeOverlayForModeActivation sizes the overlay to the active monitor.
func (h *Handler) resizeOverlayForModeActivation() {
	if h.overlayManager != nil {
		h.overlayManager.ResizeToActiveScreen()
	}
}

// resizeIndicatorOverlays sizes small indicator overlays before polling starts.
func (h *Handler) resizeIndicatorOverlays() {
	if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
		ind.ResizeToActiveScreen()
	}

	if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
		stickyInd.ResizeToActiveScreen()
	}
}
