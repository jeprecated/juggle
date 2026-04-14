package cli

import (
	"time"
)

// WakeChecker polls for the wake signal file in the background and notifies
// via WakeCh when detected. The main loop selects on WakeCh to skip delays.
type WakeChecker struct {
	effectiveID string
	WakeCh      chan struct{}
	done        chan struct{}
}

func NewWakeChecker(effectiveID string) *WakeChecker {
	return &WakeChecker{
		effectiveID: effectiveID,
		WakeCh:      make(chan struct{}, 1),
		done:        make(chan struct{}),
	}
}

func (w *WakeChecker) Run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if CheckWake(w.effectiveID) {
				select {
				case w.WakeCh <- struct{}{}:
				default:
				}
			}
		case <-w.done:
			return
		}
	}
}

func (w *WakeChecker) Stop() {
	close(w.done)
}
