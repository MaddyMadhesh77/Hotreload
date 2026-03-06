package debouncer_test

import (
	"testing"
	"time"

	"hotreload/internal/debouncer"
)

// TestSingleTrigger verifies that one Trigger call eventually produces one signal.
func TestSingleTrigger(t *testing.T) {
	d := debouncer.New(50 * time.Millisecond)
	defer d.Stop()

	d.Trigger()

	select {
	case <-d.C():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected a signal but timed out")
	}
}

// TestBurstCollapseToOneSignal verifies that a burst of rapid Trigger calls
// results in exactly one signal after the quiet period, not many.
func TestBurstCollapseToOneSignal(t *testing.T) {
	d := debouncer.New(80 * time.Millisecond)
	defer d.Stop()

	// Send 20 triggers in quick succession.
	for i := 0; i < 20; i++ {
		d.Trigger()
		time.Sleep(5 * time.Millisecond)
	}

	// Should receive exactly one signal after the quiet period.
	select {
	case <-d.C():
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected at least one debounced signal, got none")
	}

	// There should be no second signal immediately after.
	select {
	case <-d.C():
		t.Fatal("received unexpected second signal after burst")
	case <-time.After(200 * time.Millisecond):
		// expected: no extra signal
	}
}

// TestTimerResetOnSubsequentTrigger verifies that triggering during the wait
// period resets the timer, delaying the output signal.
func TestTimerResetOnSubsequentTrigger(t *testing.T) {
	d := debouncer.New(80 * time.Millisecond)
	defer d.Stop()

	start := time.Now()
	d.Trigger()
	// Trigger again 40ms later (before the first timer would fire).
	time.Sleep(40 * time.Millisecond)
	d.Trigger()

	// Signal should arrive ~80ms after the second trigger, i.e. >= 120ms total.
	select {
	case <-d.C():
		elapsed := time.Since(start)
		if elapsed < 100*time.Millisecond {
			t.Errorf("signal arrived too early (%v), timer was not reset", elapsed)
		}
	case <-time.After(600 * time.Millisecond):
		t.Fatal("expected signal but timed out")
	}
}

// TestStopCancelsTimer verifies that Stop prevents any outstanding signal.
func TestStopCancelsTimer(t *testing.T) {
	d := debouncer.New(100 * time.Millisecond)

	d.Trigger()
	d.Stop() // cancel before timer fires

	select {
	case <-d.C():
		t.Fatal("received signal after Stop was called")
	case <-time.After(300 * time.Millisecond):
		// expected: no signal
	}
}
