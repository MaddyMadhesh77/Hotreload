// Package debouncer collapses bursts of file-system events into a single
// trigger. Editors (VS Code, vim, IntelliJ) often emit several write/rename
// events for a single logical "save". Without debouncing, each one would
// kick off a separate build, causing unnecessary churn.
//
// The debouncer waits for a configurable quiet period (default 200 ms) after
// the last event before firing. This keeps the tool feeling responsive while
// avoiding redundant rebuilds.
package debouncer

import (
	"sync"
	"time"
)

// Debouncer coalesces high-frequency signals into a single output event after
// a quiet period of Duration with no new inputs.
type Debouncer struct {
	Duration time.Duration

	mu    sync.Mutex
	timer *time.Timer
	out   chan struct{}
}

// New creates a Debouncer that waits for d of silence before emitting a signal.
func New(d time.Duration) *Debouncer {
	return &Debouncer{
		Duration: d,
		out:      make(chan struct{}, 1),
	}
}

// Trigger notifies the debouncer that an event has occurred. If the internal
// timer is already running it is reset; otherwise a new timer is started.
// This call is safe for concurrent use.
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Reset(d.Duration)
		return
	}

	d.timer = time.AfterFunc(d.Duration, func() {
		d.mu.Lock()
		d.timer = nil
		d.mu.Unlock()

		// Non-blocking send: if there is already a pending signal in the
		// buffered channel, there is no need to enqueue another one.
		select {
		case d.out <- struct{}{}:
		default:
		}
	})
}

// C returns the channel that receives a signal after each debounced quiet period.
// Consumers should range over this channel or select on it.
func (d *Debouncer) C() <-chan struct{} {
	return d.out
}

// Stop cancels any pending timer. Safe to call multiple times.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
