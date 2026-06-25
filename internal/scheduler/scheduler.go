package scheduler

import (
	"context"
	"log/slog"
	"time"

	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/overdue/internal/deadline"
	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	overduenotify "github.com/containeroo/overdue/internal/notify"
)

// CheckInMonitor exposes the monitor behavior managed by the scheduler.
type CheckInMonitor interface {
	CheckInName() string
	RecordCheckIn(at time.Time) monitor.RecordResult
	Snapshot() monitor.Snapshot
	Check(now time.Time) monitor.CheckResult
	NextDeadline() (deadline time.Time, active bool)
}

// Scheduler advances a check-in monitor and enqueues lifecycle notifications.
type Scheduler struct {
	monitor           CheckInMonitor
	notifier          kit.Notifier
	resolvedReceivers []kit.ReceiverID
	logger            *slog.Logger
	metrics           *metrics.Registry
	rescheduleCh      chan struct{}
}

// New creates a scheduler for a check-in monitor and notifykit manager.
func New(
	monitor CheckInMonitor,
	notifier kit.Notifier,
	resolvedReceivers []kit.ReceiverID,
	registry *metrics.Registry,
	logger *slog.Logger,
) *Scheduler {
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
		monitor:           monitor,
		notifier:          notifier,
		resolvedReceivers: append([]kit.ReceiverID(nil), resolvedReceivers...),
		logger:            logger,
		metrics:           registry,
		rescheduleCh:      make(chan struct{}, 1),
	}
	registry.SetMonitorSnapshot(monitor.CheckInName(), monitor.Snapshot())
	return scheduler
}

// CheckInName returns the configured check-in monitor name.
func (s *Scheduler) CheckInName() string {
	return s.monitor.CheckInName()
}

// RecordCheckIn records a check-in and enqueues any resolved notification.
func (s *Scheduler) RecordCheckIn(at time.Time) monitor.RecordResult {
	result := s.monitor.RecordCheckIn(at)
	if result.ShouldNotify {
		s.enqueue(context.Background(), result.Event)
	}
	s.metrics.SetMonitorSnapshot(s.monitor.CheckInName(), result.Snapshot)
	s.requestReschedule()
	return result
}

// Snapshot returns the current check-in monitor state.
func (s *Scheduler) Snapshot() monitor.Snapshot {
	return s.monitor.Snapshot()
}

// Run starts the scheduler loop in a background goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	go s.run(ctx)
}

// Check advances the monitor and enqueues due lifecycle events.
func (s *Scheduler) Check(now time.Time) {
	result := s.monitor.Check(now)
	if result.ShouldNotify {
		s.enqueue(context.Background(), result.Event)
	}
	s.metrics.SetMonitorSnapshot(s.monitor.CheckInName(), s.monitor.Snapshot())
}

// run advances the monitor whenever the next deadline or a reschedule fires.
func (s *Scheduler) run(ctx context.Context) {
	var timer deadline.Timer
	defer timer.Stop()

	for {
		timer.Sync(s.monitor.NextDeadline())

		select {
		case <-ctx.Done():
			return
		case <-s.rescheduleCh:
			timer.Stop()
			s.Check(time.Now())
		case now := <-timer.C():
			s.Check(now)
		}
	}
}

// enqueue converts a monitor event into a notifykit notification and queues it.
func (s *Scheduler) enqueue(ctx context.Context, monitorEvent monitor.Event) {
	receiverIDs, ok := overduenotify.ReceiverIDsForEvent(monitorEvent, s.resolvedReceivers)
	if !ok {
		s.logger.Info(
			"notification skipped",
			"incidentID", monitorEvent.IncidentID,
			"notificationID", monitorEvent.NotificationID,
			"status", monitorEvent.Status,
		)
		return
	}

	id, err := s.notifier.Enqueue(ctx, overduenotify.NewEvent(monitorEvent, receiverIDs))
	if err != nil {
		s.logger.Error(
			"notification queue failed",
			"incidentID", monitorEvent.IncidentID,
			"notificationID", monitorEvent.NotificationID,
			"status", monitorEvent.Status,
			"error", err,
		)
		return
	}
	if id == "" {
		s.logger.Info(
			"notification skipped",
			"incidentID", monitorEvent.IncidentID,
			"notificationID", monitorEvent.NotificationID,
			"status", monitorEvent.Status,
		)
		return
	}
	s.logger.Info(
		"notification queued",
		"queueID", id,
		"incidentID", monitorEvent.IncidentID,
		"notificationID", monitorEvent.NotificationID,
		"status", monitorEvent.Status,
	)
}

// requestReschedule wakes the scheduler loop without blocking.
func (s *Scheduler) requestReschedule() {
	select {
	case s.rescheduleCh <- struct{}{}:
	default:
	}
}
