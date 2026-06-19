package deadline

import "time"

// Timer manages a lazily-created, resettable one-shot deadline timer.
//
// The timer is allocated only while active. Stop clears the timer reference so
// the runtime can recover it when no deadline is active.
type Timer struct {
	timer *time.Timer
}

// Sync arms the timer for deadline when active is true, or stops it otherwise.
func (t *Timer) Sync(deadline time.Time, active bool) {
	if !active {
		t.Stop()
		return
	}

	t.ResetAt(deadline)
}

// ResetAt arms the timer to fire at deadline.
func (t *Timer) ResetAt(deadline time.Time) {
	t.Reset(time.Until(deadline))
}

// Reset arms the timer to fire after d.
func (t *Timer) Reset(d time.Duration) {
	if d < 0 {
		d = 0
	}

	if t.timer == nil {
		t.timer = time.NewTimer(d)
		return
	}

	// Go 1.23+ guarantees that Reset cannot deliver a stale value from the
	// previous timer configuration, so no Stop/drain dance is needed here.
	t.timer.Reset(d)
}

// Stop disarms the timer and releases the timer reference.
func (t *Timer) Stop() {
	if t.timer == nil {
		return
	}

	// Stop prevents future delivery. In Go 1.23+, a receive after Stop returns
	// cannot observe a stale timer value.
	t.timer.Stop()
	t.timer = nil
}

// C returns the timer channel while active.
//
// A nil channel is ignored by select, which lets callers keep one simple select
// regardless of whether a deadline is active.
func (t *Timer) C() <-chan time.Time {
	if t.timer == nil {
		return nil
	}
	return t.timer.C
}
