package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/delivery"
)

const (
	defaultInitialNotificationBackoff = time.Second
	defaultMaxNotificationBackoff     = time.Minute
)

// deliveryKey identifies one notification delivery across retry attempts.
type deliveryKey string

// Fanout sends each notification event to every configured notifier.
type Fanout struct {
	mu             sync.Mutex
	Notifiers      []delivery.Notifier
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

// targetState tracks the latest known delivery status for one configured notifier.
type targetState struct {
	status          delivery.DeliveryStatus
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

// New creates a stateful fan-out notifier.
func New(notifiers []delivery.Notifier) *Fanout {
	return &Fanout{
		Notifiers:      append([]delivery.Notifier(nil), notifiers...),
		eventStates:    make(map[deliveryKey]*eventState),
		targetStates:   make([]targetState, len(notifiers)),
		InitialBackoff: defaultInitialNotificationBackoff,
		MaxBackoff:     defaultMaxNotificationBackoff,
	}
}

// NotificationStatus returns aggregate and per-target delivery state.
func (f *Fanout) NotificationStatus() delivery.Status {
	if f == nil {
		return delivery.Status{Status: delivery.StatusIdle}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.ensureTargetStatesLocked()

	targets := make([]delivery.TargetStatus, 0, len(f.Notifiers))
	status := delivery.Status{
		Total: len(f.Notifiers),
	}

	for i, notifier := range f.Notifiers {
		state := f.targetStates[i]
		targetStatus := state.status
		if targetStatus == "" {
			targetStatus = delivery.StatusIdle
		}

		switch targetStatus {
		case delivery.StatusDelivered:
			status.Delivered++
		case delivery.StatusFailed:
			status.Failed++
			status.Pending++
		case delivery.StatusPending:
			status.Pending++
		case delivery.StatusSkipped:
			status.Skipped++
		}

		targets = append(targets, delivery.TargetStatus{
			Type:            notificationTargetType(notifier),
			Name:            notificationTargetName(notifier, i),
			Status:          targetStatus,
			LastAttemptAt:   timePtrIfNonZero(state.lastAttemptAt),
			LastDeliveredAt: timePtrIfNonZero(state.lastDeliveredAt),
		})
	}

	status.Status = aggregateNotificationStatus(status)
	status.Targets = targets
	return status
}

// Notify sends the event to every not-yet-delivered notifier and joins any errors.
func (f *Fanout) Notify(ctx context.Context, event monitor.Event) error {
	if f == nil {
		return nil
	}

	key, err := delivery.NotificationKey(event)
	if err != nil {
		return err
	}
	dkey := deliveryKey(key)

	f.mu.Lock()
	state := f.eventStateLocked(dkey)
	deliveredBefore := state.deliveredCount()
	pending := f.pendingNotifiersLocked(state)
	f.mu.Unlock()

	if len(pending) == 0 {
		// A previous retry attempt may have delivered the final pending notifier.
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
			if errors.Is(err, delivery.ErrSkipped) {
				f.markSkipped(dkey, item.index, attemptAt)
				deliveredNow++
				continue
			}

			f.markFailed(item.index)
			errs = append(errs, fmt.Errorf("notifier %d: %w", item.index, err))
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

// deliveredCount returns the number of successful notifier deliveries.
func (s *eventState) deliveredCount() int {
	count := 0
	for _, delivered := range s.delivered {
		if delivered {
			count++
		}
	}
	return count
}

// pendingNotifier describes one notifier still pending for an event.
type pendingNotifier struct {
	index    int
	notifier delivery.Notifier
}

// pendingNotifiersLocked returns notifiers that have not yet succeeded for the event.
func (f *Fanout) pendingNotifiersLocked(state *eventState) (pending []pendingNotifier) {
	pending = make([]pendingNotifier, 0, len(f.Notifiers))

	for i, notifier := range f.Notifiers {
		if state.delivered[i] {
			continue
		}
		pending = append(pending, pendingNotifier{
			index:    i,
			notifier: notifier,
		})
	}

	return pending
}

// markAttempted records that a notifier delivery attempt started.
func (f *Fanout) markAttempted(index int, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ensureTargetStatesLocked()
	if index >= len(f.targetStates) {
		return
	}

	f.targetStates[index].status = delivery.StatusPending
	f.targetStates[index].lastAttemptAt = at
}

// markDelivered records a successful notifier for an event.
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
	f.targetStates[index].status = delivery.StatusDelivered
	f.targetStates[index].lastAttemptAt = at
	f.targetStates[index].lastDeliveredAt = at
}

// markSkipped records an intentionally skipped notifier for an event.
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
	f.targetStates[index].status = delivery.StatusSkipped
	f.targetStates[index].lastAttemptAt = at
	f.targetStates[index].lastDeliveredAt = time.Time{}
}

// markFailed records a failed notifier attempt.
func (f *Fanout) markFailed(index int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ensureTargetStatesLocked()
	if index >= len(f.targetStates) {
		return
	}
	f.targetStates[index].status = delivery.StatusFailed
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
		// Allocate one delivery slot per currently configured notifier.
		// Each slot tracks whether that notifier already delivered this notification successfully.
		state = &eventState{delivered: make([]bool, len(f.Notifiers))}
		f.eventStates[key] = state
		return state
	}

	if len(state.delivered) < len(f.Notifiers) {
		// Preserve existing delivery results when notifiers are added after retry state was created.
		// New notifier slots start as false, so they are treated as pending for the next attempt.
		missing := len(f.Notifiers) - len(state.delivered)
		state.delivered = append(state.delivered, make([]bool, missing)...)
	}

	return state
}

// ensureTargetStatesLocked keeps target state aligned with the configured notifiers.
func (f *Fanout) ensureTargetStatesLocked() {
	if len(f.targetStates) >= len(f.Notifiers) {
		return
	}
	missing := len(f.Notifiers) - len(f.targetStates)
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

func aggregateNotificationStatus(status delivery.Status) delivery.DeliveryStatus {
	switch {
	case status.Total == 0:
		return delivery.StatusIdle
	case status.Pending > 0 && status.Delivered > 0:
		return delivery.StatusPartialFailure
	case status.Pending > 0:
		return delivery.StatusFailed
	case status.Delivered > 0 || status.Skipped > 0:
		return delivery.StatusDelivered
	default:
		return delivery.StatusIdle
	}
}

func notificationTargetType(notifier delivery.Notifier) string {
	targeter, ok := notifier.(delivery.Targeter)
	if !ok {
		return "unknown"
	}
	return targeter.NotificationTarget().Type
}

func notificationTargetName(notifier delivery.Notifier, index int) string {
	targeter, ok := notifier.(delivery.Targeter)
	if !ok {
		return fmt.Sprintf("notifier-%d", index)
	}
	target := targeter.NotificationTarget()
	if strings.TrimSpace(target.Name) == "" {
		return fmt.Sprintf("%s-%d", target.Type, index)
	}
	return target.Name
}

func timePtrIfNonZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
