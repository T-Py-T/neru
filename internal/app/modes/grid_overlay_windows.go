//go:build windows

// internal/app/modes/grid_overlay_windows.go
// Windows grid overlay presentation; releases h.mu during Win32 UI marshaling.
// Does not implement GDI drawing; delegates to manager_windows backends.

package modes

import (
	"go.uber.org/zap"

	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

// presentGridOverlay clears, draws, resizes, and shows on the Win32 UI thread
// without holding h.mu (keyboard hook shares the same lock).
func (h *Handler) presentGridOverlay(gridInstance *domainGrid.Grid) error {
	var drawErr error

	h.runOverlaySession(func() {
		h.overlayManager.Clear()

		drawErr = h.renderer.DrawGrid(gridInstance, "")
		if drawErr != nil {
			return
		}

		h.overlayManager.ResizeToActiveScreen()
		h.overlayManager.Show()
	})

	return drawErr
}

// refreshGridOverlayInput redraws grid match state during navigation.
func (h *Handler) refreshGridOverlayInput(
	gridInstance *domainGrid.Grid,
	input string,
	forceRedraw bool,
) {
	h.runOverlaySession(func() {
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
	})
}

// clearAndHideOverlaySurface clears and hides the overlay manager.
func (h *Handler) clearAndHideOverlaySurface() {
	h.runOverlaySession(func() {
		h.overlayManager.Clear()
		h.overlayManager.Hide()
	})
}

// resizeOverlayForModeActivation sizes the overlay to the active monitor.
func (h *Handler) resizeOverlayForModeActivation() {
	if h.overlayManager == nil {
		return
	}

	h.runOverlaySession(func() {
		h.overlayManager.ResizeToActiveScreen()
	})
}

// resizeIndicatorOverlays sizes small indicator overlays before polling starts.
func (h *Handler) resizeIndicatorOverlays() {
	h.runOverlaySession(func() {
		if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
			ind.ResizeToActiveScreen()
		}

		if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
			stickyInd.ResizeToActiveScreen()
		}
	})
}
