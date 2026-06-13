//go:build !windows

// internal/app/modes/indicator_stop_default.go
// Default indicator poller shutdown; caller holds h.mu throughout.

package modes

// waitForIndicatorPoller blocks until the indicator polling goroutine exits.
// Caller must hold h.mu.
func (h *Handler) waitForIndicatorPoller(doneCh <-chan struct{}) {
	if doneCh != nil {
		<-doneCh
	}
}
