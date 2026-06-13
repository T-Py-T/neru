//go:build windows

// internal/app/modes/indicator_stop_windows.go
// Windows indicator poller shutdown; releases h.mu while waiting on the poller.

package modes

// waitForIndicatorPoller blocks until the indicator polling goroutine exits.
// Caller must hold h.mu; the lock is released for the duration of the wait.
func (h *Handler) waitForIndicatorPoller(doneCh <-chan struct{}) {
	if doneCh == nil {
		return
	}

	h.mu.Unlock()
	<-doneCh
	h.mu.Lock()
}
