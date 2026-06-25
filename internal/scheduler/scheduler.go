package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/containeroo/overdue/internal/deadline"
	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
)

// CheckInMonitor exposes the monitor behavior managed by the scheduler.
type CheckInMonitor interface {
	CheckInName() string
	RecordCheckIn(at time.Time) monitor.RecordResult
	Snapshot() monitor.Snapshot
	Check(now time.Time) monitor.CheckResult
	NextDeadline() (deadline time.Time, active bool)
}

// retryAfterError is implemented by notification errors that request delayed retry.
type retryAfterError interface {
	error
	RetryAfter() time.Duration
}

// notificationStatsError is implemented by notification errors that expose retry batch stats.
type notificationStatsError interface {
	error
	NotificationStats() (delivered int, failed int, pending int)
}

const defaultNotificationRetryAfter = time.Second

// Scheduler advances a check-in monitor and delivers emitted lifecycle events.
type Scheduler struct {
	mu           sync.Mutex
	monitor      CheckInMonitor
	notifier     target.Dispatcher
	logger       *slog.Logger
	metrics      *metrics.Registry
	rescheduleCh chan struct{}
	pending      map[string]pendingDelivery
}

type pendingDelivery struct {
	event   monitor.Event
	retryAt time.Time
}

// New creates a notification scheduler for a check-in monitor.
func New(monitor CheckInMonitor, notifier target.Dispatcher, registry *metrics.Registry, logger *slog.Logger) *Scheduler {
	if monitor == nil {
		panic("check-in monitor must not be nil")
	}
	if notifier == nil {
		panic("scheduler notifier must not be nil")
	}
	if registry == nil {
		panic("scheduler metrics registry must not be nil")
	}
	if logger == nil {
		panic("scheduler logger must not be nil")
	}

	scheduler := &Scheduler{
		monitor:      monitor,
		notifier:     notifier,
		logger:       logger,
		metrics:      registry,
		rescheduleCh: make(chan struct{}, 1),
		pending:      make(map[string]pendingDelivery),
	}
	registry.SetMonitorSnapshot(monitor.CheckInName(), monitor.Snapshot())
	return scheduler
}

// CheckInName returns the configured check-in monitor name.
func (s *Scheduler) CheckInName() string {
	return s.monitor.CheckInName()
}

// RecordCheckIn records a check-in and schedules any resolved notification for target.
func (s *Scheduler) RecordCheckIn(at time.Time) monitor.RecordResult {
	result := s.monitor.RecordCheckIn(at)
	if result.ShouldNotify {
		s.enqueue(result.Event, time.Time{})
	}
	s.metrics.SetMonitorSnapshot(s.monitor.CheckInName(), result.Snapshot)
	s.requestReschedule()
	return result
}

// Snapshot returns the current check-in monitor state.
func (s *Scheduler) Snapshot() monitor.Snapshot {
	return s.monitor.Snapshot()
}

// NotificationStatus returns aggregate notification delivery state.
func (s *Scheduler) NotificationStatus() target.Status {
	provider, ok := s.notifier.(target.StatusProvider)
	if !ok {
		return target.Status{Status: target.StatusIdle}
	}
	return provider.NotificationStatus()
}

// Run starts the scheduler loop in a background goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	go s.run(ctx)
}

// Check advances the monitor and delivers due notification events.
func (s *Scheduler) Check(ctx context.Context, now time.Time) error {
	result := s.monitor.Check(now)
	if result.ShouldNotify {
		s.enqueue(result.Event, time.Time{})
	}
	s.metrics.SetMonitorSnapshot(s.monitor.CheckInName(), s.monitor.Snapshot())
	return s.deliverDue(ctx, now)
}

// run waits for monitor deadlines, retry deadlines, reschedule requests, or shutdown.
func (s *Scheduler) run(ctx context.Context) {
	var timer deadline.Timer
	defer timer.Stop()

	for {
		// Synchronize the timer with the current scheduler state.
		// If no monitor or retry deadline is active, the timer is stopped and
		// timer.C() returns nil, which disables that select case.
		timer.Sync(s.nextDeadline())

		select {
		case <-ctx.Done():
			// Shutdown is owned by the context. The deferred Stop releases the timer.
			return

		case <-s.rescheduleCh:
			// Scheduler state changed. Stop the old timer, check immediately,
			// then recompute the next monitor or retry deadline.
			timer.Stop()
			_ = s.Check(ctx, time.Now())
			continue

		case now := <-timer.C():
			// The active monitor or retry deadline fired.
			_ = s.Check(ctx, now)
		}
	}
}

// nextDeadline returns the earliest monitor or retry deadline.
func (s *Scheduler) nextDeadline() (deadline time.Time, active bool) {
	monitorDeadline, monitorActive := s.monitor.NextDeadline()
	retryDeadline, retryActive := s.nextRetryDeadline()

	switch {
	case monitorActive && retryActive:
		return earlierTime(monitorDeadline, retryDeadline), true
	case monitorActive:
		return monitorDeadline, true
	case retryActive:
		return retryDeadline, true
	default:
		return time.Time{}, false
	}
}

// enqueue stores an event for target or retry.
func (s *Scheduler) enqueue(event monitor.Event, retryAt time.Time) {
	key, err := target.NotificationKey(event)
	if err != nil {
		s.logger.Warn(
			"notification event missing notification id; delivery skipped",
			"incidentID", incidentID(event),
			"status", event.Status,
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if retryAt.IsZero() {
		retryAt = event.Now
	}
	s.pending[key] = pendingDelivery{event: event, retryAt: retryAt}
}

// deliverDue delivers notifications due at now.
func (s *Scheduler) deliverDue(ctx context.Context, now time.Time) error {
	due := s.dueDeliveries(now)
	var errs []error

	for _, delivery := range due {
		if err := s.deliver(ctx, delivery.event, now); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// dueDeliveries returns pending deliveries due at now.
func (s *Scheduler) dueDeliveries(now time.Time) []pendingDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()

	due := make([]pendingDelivery, 0, len(s.pending))
	for _, delivery := range s.pending {
		if delivery.retryAt.IsZero() || !now.Before(delivery.retryAt) {
			due = append(due, delivery)
		}
	}
	return due
}

// deliver sends one event and schedules retry on failure.
func (s *Scheduler) deliver(ctx context.Context, event monitor.Event, now time.Time) error {
	if err := s.notifier.Notify(ctx, event); err != nil {
		s.recordNotificationMetrics()
		retryAfter := notificationRetryAfter(err)
		retryAt := now.Add(retryAfter)
		delivered, failed, pending := notificationStats(err)

		s.enqueue(event, retryAt)
		s.logger.Warn(
			"notification failed; retry scheduled",
			"incidentID", event.IncidentID,
			"notificationID", event.NotificationID,
			"status", event.Status,
			"delivered", delivered,
			"failed", failed,
			"pending", pending,
			"retryAfter", retryAfter.String(),
			"retryAt", retryAt,
			"error", err,
		)
		s.requestReschedule()
		return err
	}

	s.recordNotificationMetrics()
	s.clear(event)
	s.logger.Info(
		"notification batch completed",
		"incidentID", event.IncidentID,
		"notificationID", event.NotificationID,
		"status", event.Status,
	)
	return nil
}

// clear removes delivered notification state.
func (s *Scheduler) clear(event monitor.Event) {
	key, err := target.NotificationKey(event)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, key)
}

// nextRetryDeadline returns the next pending retry time.
func (s *Scheduler) nextRetryDeadline() (deadline time.Time, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, delivery := range s.pending {
		if delivery.retryAt.IsZero() {
			return time.Now(), true
		}
		if !active || delivery.retryAt.Before(deadline) {
			deadline = delivery.retryAt
			active = true
		}
	}
	return deadline, active
}

// recordNotificationMetrics updates metrics from the current notifier target status.
func (s *Scheduler) recordNotificationMetrics() {
	provider, ok := s.notifier.(target.StatusProvider)
	if !ok {
		return
	}

	s.metrics.SetNotificationStatus(provider.NotificationStatus())
}

// requestReschedule signals the scheduler loop to re-check state and recompute its timer.
func (s *Scheduler) requestReschedule() {
	select {
	case s.rescheduleCh <- struct{}{}:
	default:
	}
}

// earlierTime returns the earlier of two times.
func earlierTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// notificationRetryAfter returns the requested retry delay from a notification error.
func notificationRetryAfter(err error) time.Duration {
	if retryErr, ok := errors.AsType[retryAfterError](err); ok {
		if retryAfter := retryErr.RetryAfter(); retryAfter > 0 {
			return retryAfter
		}
	}
	return defaultNotificationRetryAfter
}

// notificationStats returns target stats from a notification error.
func notificationStats(err error) (delivered, failed, pending int) {
	if statsErr, ok := errors.AsType[notificationStatsError](err); ok {
		return statsErr.NotificationStats()
	}
	return 0, 1, 1
}

// incidentID returns the event incident ID or a fallback label for defensive logs.
func incidentID(event monitor.Event) string {
	if strings.TrimSpace(event.IncidentID) == "" {
		return "unknown"
	}
	return event.IncidentID
}
