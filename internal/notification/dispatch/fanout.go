package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
)

const (
	defaultInitialNotificationBackoff = time.Second
	defaultMaxNotificationBackoff     = time.Minute
)

// deliveryKey identifies one notification delivery across retry attempts.
type deliveryKey string

// Fanout sends each notification event to every configured target.
type Fanout struct {
	mu             sync.Mutex
	targets        []target.Notifier
	eventStates    map[deliveryKey]*eventState
	targetStates   []targetState
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// eventState tracks retry progress for one notification event.
type eventState struct {
	delivered []bool
	attempts  int
}

// targetState tracks the latest known target status for one configured target.
type targetState struct {
	status          target.DeliveryStatus
	lastAttemptAt   time.Time
	lastDeliveredAt time.Time
}

// RetryError wraps notification failures and tells the scheduler when to retry.
type RetryError struct {
	Err       error
	RetryWait time.Duration
	Delivered int
	Failed    int
	Pending   int
}

// Error returns the wrapped error string.
func (e *RetryError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e *RetryError) Unwrap() error {
	return e.Err
}

// RetryAfter returns the delay before the next retry attempt.
func (e *RetryError) RetryAfter() time.Duration {
	return e.RetryWait
}

// NotificationStats returns retry batch stats.
func (e *RetryError) NotificationStats() (delivered, failed, pending int) {
	return e.Delivered, e.Failed, e.Pending
}

// New creates a stateful target fan-out notifier.
func New(targets []target.Notifier) *Fanout {
	return &Fanout{
		targets:        append([]target.Notifier(nil), targets...),
		eventStates:    make(map[deliveryKey]*eventState),
		targetStates:   make([]targetState, len(targets)),
		InitialBackoff: defaultInitialNotificationBackoff,
		MaxBackoff:     defaultMaxNotificationBackoff,
	}
}

// Targets returns the configured notification target metadata in delivery order.
func (f *Fanout) Targets() []target.Target {
	if f == nil {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	targets := make([]target.Target, 0, len(f.targets))
	for i, notifier := range f.targets {
		targets = append(targets, targetMetadata(notifier, i))
	}
	return targets
}

// NotificationStatus returns aggregate and per-target delivery state.
func (f *Fanout) NotificationStatus() target.Status {
	if f == nil {
		return target.Status{Status: target.StatusIdle}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.ensureTargetStatesLocked()

	targets := make([]target.TargetStatus, 0, len(f.targets))
	status := target.Status{Total: len(f.targets)}

	for i, notifier := range f.targets {
		state := f.targetStates[i]
		targetStatus := state.status
		if targetStatus == "" {
			targetStatus = target.StatusIdle
		}

		switch targetStatus {
		case target.StatusDelivered:
			status.Delivered++
		case target.StatusFailed:
			status.Failed++
			status.Pending++
		case target.StatusPending:
			status.Pending++
		case target.StatusSkipped:
			status.Skipped++
		}

		metadata := targetMetadata(notifier, i)
		targets = append(targets, target.TargetStatus{
			Type:            metadata.Type,
			Name:            metadata.Name,
			Status:          targetStatus,
			LastAttemptAt:   timePtrIfNonZero(state.lastAttemptAt),
			LastDeliveredAt: timePtrIfNonZero(state.lastDeliveredAt),
		})
	}

	status.Status = aggregateNotificationStatus(status)
	status.Targets = targets
	return status
}

// Notify sends the event to every not-yet-delivered target and joins any errors.
func (f *Fanout) Notify(ctx context.Context, event monitor.Event) error {
	if f == nil {
		return nil
	}

	key, err := target.NotificationKey(event)
	if err != nil {
		return err
	}
	dkey := deliveryKey(key)

	f.mu.Lock()
	state := f.eventStateLocked(dkey)
	deliveredBefore := state.deliveredCount()
	pending := f.pendingTargetsLocked(state)
	f.mu.Unlock()

	if len(pending) == 0 {
		// A previous retry attempt may have delivered the final pending target.
		f.clearEvent(dkey)
		return nil
	}

	errs := make([]error, 0)
	deliveredNow := 0

	for _, item := range pending {
		attemptAt := time.Now()
		f.markAttempted(item.index, attemptAt)

		if item.notifier == nil {
			f.markDelivered(dkey, item.index, attemptAt)
			deliveredNow++
			continue
		}

		if err := item.notifier.Notify(ctx, event); err != nil {
			if errors.Is(err, target.ErrSkipped) {
				f.markSkipped(dkey, item.index, attemptAt)
				deliveredNow++
				continue
			}

			f.markFailed(item.index)
			metadata := targetMetadata(item.notifier, item.index)
			errs = append(errs, fmt.Errorf("%s %q: %w", metadata.Type, metadata.Name, err))
			continue
		}

		f.markDelivered(dkey, item.index, attemptAt)
		deliveredNow++
	}

	if len(errs) == 0 {
		f.clearEvent(dkey)
		return nil
	}

	retryAfter := f.nextBackoff(dkey)
	return &RetryError{
		Err:       errors.Join(errs...),
		RetryWait: retryAfter,
		Delivered: deliveredBefore + deliveredNow,
		Failed:    len(errs),
		Pending:   len(errs),
	}
}

// deliveredCount returns the number of successful target deliveries.
func (s *eventState) deliveredCount() int {
	count := 0
	for _, delivered := range s.delivered {
		if delivered {
			count++
		}
	}
	return count
}

// pendingTarget describes one target still pending for an event.
type pendingTarget struct {
	index    int
	notifier target.Notifier
}

// pendingTargetsLocked returns targets that have not yet succeeded for the event.
func (f *Fanout) pendingTargetsLocked(state *eventState) (pending []pendingTarget) {
	pending = make([]pendingTarget, 0, len(f.targets))

	for i, notifier := range f.targets {
		if state.delivered[i] {
			continue
		}
		pending = append(pending, pendingTarget{
			index:    i,
			notifier: notifier,
		})
	}

	return pending
}

// markAttempted records that a target delivery attempt started.
func (f *Fanout) markAttempted(index int, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ensureTargetStatesLocked()
	if index >= len(f.targetStates) {
		return
	}

	f.targetStates[index].status = target.StatusPending
	f.targetStates[index].lastAttemptAt = at
}

// markDelivered records a successful target for an event.
func (f *Fanout) markDelivered(key deliveryKey, index int, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	state := f.eventStateLocked(key)
	if index >= len(state.delivered) {
		return
	}
	state.delivered[index] = true

	f.ensureTargetStatesLocked()
	if index >= len(f.targetStates) {
		return
	}
	f.targetStates[index].status = target.StatusDelivered
	f.targetStates[index].lastAttemptAt = at
	f.targetStates[index].lastDeliveredAt = at
}

// markSkipped records an intentionally skipped target for an event.
func (f *Fanout) markSkipped(key deliveryKey, index int, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	state := f.eventStateLocked(key)
	if index >= len(state.delivered) {
		return
	}
	state.delivered[index] = true

	f.ensureTargetStatesLocked()
	if index >= len(f.targetStates) {
		return
	}
	f.targetStates[index].status = target.StatusSkipped
	f.targetStates[index].lastAttemptAt = at
	f.targetStates[index].lastDeliveredAt = time.Time{}
}

// markFailed records a failed target attempt.
func (f *Fanout) markFailed(index int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ensureTargetStatesLocked()
	if index >= len(f.targetStates) {
		return
	}
	f.targetStates[index].status = target.StatusFailed
}

// clearEvent removes retry state once an event is fully delivered.
func (f *Fanout) clearEvent(key deliveryKey) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.eventStates, key)
}

// nextBackoff returns the next exponential backoff delay for an event.
func (f *Fanout) nextBackoff(key deliveryKey) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()

	state := f.eventStateLocked(key)

	state.attempts++
	attempt := state.attempts

	backoff := f.initialBackoffLocked()
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= f.maxBackoffLocked() {
			return f.maxBackoffLocked()
		}
	}

	if backoff > f.maxBackoffLocked() {
		return f.maxBackoffLocked()
	}
	return backoff
}

// eventStateLocked returns initialized retry state for one notification delivery.
func (f *Fanout) eventStateLocked(key deliveryKey) *eventState {
	if f.eventStates == nil {
		f.eventStates = make(map[deliveryKey]*eventState)
	}
	f.ensureTargetStatesLocked()

	state := f.eventStates[key]
	if state == nil {
		// Allocate one target slot per currently configured target.
		// Each slot tracks whether that target already delivered this notification successfully.
		state = &eventState{delivered: make([]bool, len(f.targets))}
		f.eventStates[key] = state
		return state
	}

	if len(state.delivered) < len(f.targets) {
		// Preserve existing target results when targets are added after retry state was created.
		// New target slots start as false, so they are treated as pending for the next attempt.
		missing := len(f.targets) - len(state.delivered)
		state.delivered = append(state.delivered, make([]bool, missing)...)
	}

	return state
}

// ensureTargetStatesLocked keeps target state aligned with the configured targets.
func (f *Fanout) ensureTargetStatesLocked() {
	if len(f.targetStates) >= len(f.targets) {
		return
	}
	missing := len(f.targets) - len(f.targetStates)
	f.targetStates = append(f.targetStates, make([]targetState, missing)...)
}

// initialBackoffLocked returns the configured initial backoff or the default.
func (f *Fanout) initialBackoffLocked() time.Duration {
	if f.InitialBackoff <= 0 {
		return defaultInitialNotificationBackoff
	}
	return f.InitialBackoff
}

// maxBackoffLocked returns the configured maximum backoff or the default.
func (f *Fanout) maxBackoffLocked() time.Duration {
	if f.MaxBackoff <= 0 {
		return defaultMaxNotificationBackoff
	}
	return f.MaxBackoff
}

func aggregateNotificationStatus(status target.Status) target.DeliveryStatus {
	switch {
	case status.Total == 0:
		return target.StatusIdle
	case status.Pending > 0 && status.Delivered > 0:
		return target.StatusPartialFailure
	case status.Pending > 0:
		return target.StatusFailed
	case status.Delivered > 0 || status.Skipped > 0:
		return target.StatusDelivered
	default:
		return target.StatusIdle
	}
}

func targetMetadata(notifier target.Notifier, index int) target.Target {
	metadata := target.Target{Type: "unknown", Name: fmt.Sprintf("target-%d", index)}
	if notifier != nil {
		metadata = notifier.Target()
	}

	metadata.Type = strings.TrimSpace(metadata.Type)
	metadata.Name = strings.TrimSpace(metadata.Name)
	if metadata.Type == "" {
		metadata.Type = "unknown"
	}
	if metadata.Name == "" {
		metadata.Name = fmt.Sprintf("%s-%d", metadata.Type, index)
	}
	return metadata
}

func timePtrIfNonZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
