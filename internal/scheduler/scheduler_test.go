package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingNotifier struct {
	mu     sync.Mutex
	events []monitor.Event
	err    error
	called chan monitor.Event
}

// newRecordingNotifier creates a recording notifier.
func newRecordingNotifier() *recordingNotifier {
	return &recordingNotifier{called: make(chan monitor.Event, 10)}
}

// Notify records notification calls.
func (n *recordingNotifier) Notify(_ context.Context, event monitor.Event) error {
	n.mu.Lock()
	n.events = append(n.events, event)
	n.mu.Unlock()
	n.called <- event
	return n.err
}

// Events returns recorded notification events.
func (n *recordingNotifier) Events() []monitor.Event {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]monitor.Event(nil), n.events...)
}

type stubMonitor struct {
	name     string
	record   monitor.RecordResult
	snapshot monitor.Snapshot
	check    monitor.CheckResult
	deadline time.Time
	active   bool
}

// CheckInName returns the stub monitor name.
func (m *stubMonitor) CheckInName() string {
	return m.name
}

// RecordCheckIn returns the configured record result.
func (m *stubMonitor) RecordCheckIn(time.Time) monitor.RecordResult {
	return m.record
}

// Snapshot returns the configured snapshot.
func (m *stubMonitor) Snapshot() monitor.Snapshot {
	return m.snapshot
}

// Check returns the configured check result.
func (m *stubMonitor) Check(time.Time) monitor.CheckResult {
	return m.check
}

// NextDeadline returns the configured deadline state.
func (m *stubMonitor) NextDeadline() (time.Time, bool) {
	return m.deadline, m.active
}

func TestScheduler_CheckInName(t *testing.T) {
	t.Parallel()
	t.Run("returns monitor check-in name", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("prometheus", time.Minute, time.Second, testLogger())
		s := New(checkInMonitor, newRecordingNotifier(), metrics.NewRegistry(), testLogger())

		assert.Equal(t, "prometheus", s.CheckInName())
	})
}

func TestScheduler_RecordCheckIn(t *testing.T) {
	t.Parallel()
	t.Run("records check-in and wakes scheduler", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		s := New(checkInMonitor, newRecordingNotifier(), metrics.NewRegistry(), testLogger())
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		result := s.RecordCheckIn(now)

		assert.Equal(t, monitor.PhaseAwaiting, result.Snapshot.Phase)

		select {
		case <-s.rescheduleCh:
		default:
			require.Fail(t, "scheduler was not woken")
		}
	})

	t.Run("enqueues resolved notification", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
		notifier := newRecordingNotifier()
		s := New(checkInMonitor, notifier, metrics.NewRegistry(), testLogger())

		s.RecordCheckIn(start)
		require.NoError(t, s.Check(context.Background(), start.Add(time.Minute)))

		result := s.RecordCheckIn(start.Add(2 * time.Minute))

		require.True(t, result.ShouldNotify)
		assert.Equal(t, monitor.StatusResolved, result.Event.Status)
		assert.Contains(t, s.pending, result.Event.NotificationID)
	})
}

func TestScheduler_Snapshot(t *testing.T) {
	t.Parallel()
	t.Run("returns monitor snapshot", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		s := New(checkInMonitor, newRecordingNotifier(), metrics.NewRegistry(), testLogger())
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor.RecordCheckIn(now)

		snapshot := s.Snapshot()

		assert.Equal(t, monitor.PhaseAwaiting, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.Equal(now))
	})
}

func TestScheduler_Check(t *testing.T) {
	t.Parallel()
	t.Run("delivers alerting notification after alerting delay", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		notifier := newRecordingNotifier()
		checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
		checkInMonitor.RecordCheckIn(start)
		s := New(checkInMonitor, notifier, metrics.NewRegistry(), testLogger())

		require.NoError(t, s.Check(context.Background(), start.Add(time.Minute)))

		events := notifier.Events()
		require.Len(t, events, 1)
		assert.Equal(t, monitor.StatusAlerting, events[0].Status)
		assert.Equal(t, monitor.PhaseAlerting, s.Snapshot().Phase)
	})

	t.Run("retries failed alerting notification", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		notifier := newRecordingNotifier()
		notifier.err = retryAfterTestError{wait: time.Second}
		checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
		checkInMonitor.RecordCheckIn(start)
		s := New(checkInMonitor, notifier, metrics.NewRegistry(), testLogger())

		require.Error(t, s.Check(context.Background(), start.Add(time.Minute)))
		require.Error(t, s.Check(context.Background(), start.Add(time.Minute+time.Second)))

		events := notifier.Events()
		require.Len(t, events, 2)
		assert.Equal(t, events[0].NotificationID, events[1].NotificationID)
		assert.Equal(t, monitor.StatusAlerting, events[1].Status)
	})

	t.Run("retries failed resolved notification", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		notifier := newRecordingNotifier()
		checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
		s := New(checkInMonitor, notifier, metrics.NewRegistry(), testLogger())
		s.RecordCheckIn(start)

		require.NoError(t, s.Check(context.Background(), start.Add(time.Minute)))
		notifier.err = retryAfterTestError{wait: time.Second}
		s.RecordCheckIn(start.Add(2 * time.Minute))
		require.Error(t, s.Check(context.Background(), start.Add(2*time.Minute)))
		notifier.err = nil
		require.NoError(t, s.Check(context.Background(), start.Add(2*time.Minute+time.Second)))

		events := notifier.Events()
		require.Len(t, events, 3)
		assert.Equal(t, monitor.StatusAlerting, events[0].Status)
		assert.Equal(t, monitor.StatusResolved, events[1].Status)
		assert.Equal(t, monitor.StatusResolved, events[2].Status)
		assert.Equal(t, events[1].NotificationID, events[2].NotificationID)
	})
}

func TestScheduler_Run(t *testing.T) {
	t.Parallel()
	t.Run("uses expected deadline before overdue deadline", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		notifier := newRecordingNotifier()
		checkInMonitor := monitor.New("default", 30*time.Millisecond, 80*time.Millisecond, testLogger())
		s := New(checkInMonitor, notifier, metrics.NewRegistry(), testLogger())
		s.Run(ctx)
		s.RecordCheckIn(time.Now())

		require.Eventually(t, func() bool {
			return s.Snapshot().Phase == monitor.PhaseOverdue
		}, time.Second, 5*time.Millisecond)

		assert.Empty(t, notifier.Events())

		require.Eventually(t, func() bool {
			return len(notifier.Events()) == 1
		}, time.Second, 5*time.Millisecond)
		assert.Equal(t, monitor.PhaseAlerting, s.Snapshot().Phase)
	})

	t.Run("does not spin after alert", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		notifier := newRecordingNotifier()
		checkInMonitor := monitor.New("default", 10*time.Millisecond, 0, testLogger())
		s := New(checkInMonitor, notifier, metrics.NewRegistry(), testLogger())
		s.Run(ctx)
		s.RecordCheckIn(time.Now())

		require.Eventually(t, func() bool {
			return s.Snapshot().Phase == monitor.PhaseAlerting
		}, time.Second, 5*time.Millisecond)

		eventCount := len(notifier.Events())
		time.Sleep(50 * time.Millisecond)

		assert.Equal(t, eventCount, len(notifier.Events()))
		assert.Equal(t, monitor.PhaseAlerting, s.Snapshot().Phase)
	})
}

func TestScheduler_nextDeadline(t *testing.T) {
	t.Parallel()
	t.Run("returns inactive without monitor or retry deadline", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{
			monitor: &stubMonitor{},
			pending: make(map[string]pendingDelivery),
		}

		deadline, active := s.nextDeadline()

		assert.False(t, active)
		assert.True(t, deadline.IsZero())
	})

	t.Run("returns monitor deadline", func(t *testing.T) {
		t.Parallel()

		want := time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC)
		s := &Scheduler{
			monitor: &stubMonitor{deadline: want, active: true},
			pending: make(map[string]pendingDelivery),
		}

		deadline, active := s.nextDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(want))
	})

	t.Run("returns retry deadline", func(t *testing.T) {
		t.Parallel()

		want := time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC)
		s := &Scheduler{
			monitor: &stubMonitor{},
			pending: map[string]pendingDelivery{
				"notification": {event: schedulerTestEvent("notification"), retryAt: want},
			},
		}

		deadline, active := s.nextDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(want))
	})

	t.Run("returns earlier monitor deadline", func(t *testing.T) {
		t.Parallel()

		monitorDeadline := time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC)
		retryDeadline := time.Date(2026, 6, 19, 8, 2, 0, 0, time.UTC)
		s := &Scheduler{
			monitor: &stubMonitor{deadline: monitorDeadline, active: true},
			pending: map[string]pendingDelivery{
				"notification": {event: schedulerTestEvent("notification"), retryAt: retryDeadline},
			},
		}

		deadline, active := s.nextDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(monitorDeadline))
	})

	t.Run("returns earlier retry deadline", func(t *testing.T) {
		t.Parallel()

		monitorDeadline := time.Date(2026, 6, 19, 8, 2, 0, 0, time.UTC)
		retryDeadline := time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC)
		s := &Scheduler{
			monitor: &stubMonitor{deadline: monitorDeadline, active: true},
			pending: map[string]pendingDelivery{
				"notification": {event: schedulerTestEvent("notification"), retryAt: retryDeadline},
			},
		}

		deadline, active := s.nextDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(retryDeadline))
	})
}

func TestScheduler_enqueue(t *testing.T) {
	t.Parallel()
	t.Run("stores event using event time when retry time is zero", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		event := schedulerTestEvent("notification")
		event.Now = now
		s := &Scheduler{
			logger:  testLogger(),
			pending: make(map[string]pendingDelivery),
		}

		s.enqueue(event, time.Time{})

		delivery, ok := s.pending["notification"]
		require.True(t, ok)
		assert.True(t, delivery.retryAt.Equal(now))
	})

	t.Run("stores explicit retry time", func(t *testing.T) {
		t.Parallel()

		retryAt := time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC)
		event := schedulerTestEvent("notification")
		s := &Scheduler{
			logger:  testLogger(),
			pending: make(map[string]pendingDelivery),
		}

		s.enqueue(event, retryAt)

		delivery, ok := s.pending["notification"]
		require.True(t, ok)
		assert.True(t, delivery.retryAt.Equal(retryAt))
	})

	t.Run("skips event without notification id", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{
			logger:  testLogger(),
			pending: make(map[string]pendingDelivery),
		}

		s.enqueue(schedulerTestEvent(" "), time.Now())

		assert.Empty(t, s.pending)
	})
}

func TestScheduler_deliverDue(t *testing.T) {
	t.Parallel()
	t.Run("delivers only due notifications", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		notifier := newRecordingNotifier()
		s := &Scheduler{
			notifier:     notifier,
			logger:       testLogger(),
			rescheduleCh: make(chan struct{}, 1),
			pending: map[string]pendingDelivery{
				"due":    {event: schedulerTestEvent("due"), retryAt: now.Add(-time.Second)},
				"future": {event: schedulerTestEvent("future"), retryAt: now.Add(time.Second)},
			},
		}

		err := s.deliverDue(context.Background(), now)

		require.NoError(t, err)
		events := notifier.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "due", events[0].NotificationID)
		assert.NotContains(t, s.pending, "due")
		assert.Contains(t, s.pending, "future")
	})

	t.Run("joins target errors", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		notifier := newRecordingNotifier()
		notifier.err = errors.New("boom")
		s := &Scheduler{
			notifier:     notifier,
			logger:       testLogger(),
			rescheduleCh: make(chan struct{}, 1),
			pending: map[string]pendingDelivery{
				"due": {event: schedulerTestEvent("due"), retryAt: now},
			},
		}

		err := s.deliverDue(context.Background(), now)

		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})
}

func TestScheduler_dueDeliveries(t *testing.T) {
	t.Parallel()
	t.Run("returns due deliveries", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		s := &Scheduler{
			pending: map[string]pendingDelivery{
				"zero":   {event: schedulerTestEvent("zero")},
				"past":   {event: schedulerTestEvent("past"), retryAt: now.Add(-time.Second)},
				"now":    {event: schedulerTestEvent("now"), retryAt: now},
				"future": {event: schedulerTestEvent("future"), retryAt: now.Add(time.Second)},
			},
		}

		deliveries := s.dueDeliveries(now)

		assert.ElementsMatch(t, []string{"zero", "past", "now"}, deliveryIDs(deliveries))
	})
}

func TestScheduler_deliver(t *testing.T) {
	t.Parallel()
	t.Run("clears delivered event", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		event := schedulerTestEvent("notification")
		s := &Scheduler{
			notifier: newRecordingNotifier(),
			logger:   testLogger(),
			pending: map[string]pendingDelivery{
				"notification": {event: event, retryAt: now},
			},
		}

		err := s.deliver(context.Background(), event, now)

		require.NoError(t, err)
		assert.NotContains(t, s.pending, "notification")
	})

	t.Run("schedules retry on failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		event := schedulerTestEvent("notification")
		notifier := newRecordingNotifier()
		notifier.err = retryAfterTestError{wait: 2 * time.Second}
		s := &Scheduler{
			notifier:     notifier,
			logger:       testLogger(),
			rescheduleCh: make(chan struct{}, 1),
			pending:      make(map[string]pendingDelivery),
		}

		err := s.deliver(context.Background(), event, now)

		require.Error(t, err)
		delivery, ok := s.pending["notification"]
		require.True(t, ok)
		assert.True(t, delivery.retryAt.Equal(now.Add(2*time.Second)))

		select {
		case <-s.rescheduleCh:
		default:
			require.Fail(t, "scheduler was not woken")
		}
	})
}

func TestScheduler_clear(t *testing.T) {
	t.Parallel()
	t.Run("removes delivered notification state", func(t *testing.T) {
		t.Parallel()

		event := schedulerTestEvent("notification")
		s := &Scheduler{
			pending: map[string]pendingDelivery{
				"notification": {event: event},
			},
		}

		s.clear(event)

		assert.NotContains(t, s.pending, "notification")
	})

	t.Run("ignores event without notification id", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{
			pending: map[string]pendingDelivery{
				"notification": {event: schedulerTestEvent("notification")},
			},
		}

		s.clear(schedulerTestEvent(" "))

		assert.Contains(t, s.pending, "notification")
	})
}

func TestScheduler_nextRetryDeadline(t *testing.T) {
	t.Parallel()
	t.Run("returns inactive without pending deliveries", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{pending: make(map[string]pendingDelivery)}

		deadline, active := s.nextRetryDeadline()

		assert.False(t, active)
		assert.True(t, deadline.IsZero())
	})

	t.Run("returns immediate deadline for zero retry time", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{
			pending: map[string]pendingDelivery{
				"notification": {event: schedulerTestEvent("notification")},
			},
		}

		before := time.Now()
		deadline, active := s.nextRetryDeadline()
		after := time.Now()

		assert.True(t, active)
		assert.False(t, deadline.Before(before))
		assert.False(t, deadline.After(after))
	})

	t.Run("returns earliest retry deadline", func(t *testing.T) {
		t.Parallel()

		first := time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC)
		second := time.Date(2026, 6, 19, 8, 2, 0, 0, time.UTC)
		s := &Scheduler{
			pending: map[string]pendingDelivery{
				"first":  {event: schedulerTestEvent("first"), retryAt: first},
				"second": {event: schedulerTestEvent("second"), retryAt: second},
			},
		}

		deadline, active := s.nextRetryDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(first))
	})
}

func TestScheduler_recordNotificationMetrics(t *testing.T) {
	t.Parallel()
	t.Run("records notifier target status", func(t *testing.T) {
		t.Parallel()

		registry := metrics.NewRegistry()
		s := &Scheduler{
			metrics: registry,
			notifier: statusNotifier{status: target.Status{Targets: []target.TargetStatus{
				{Type: "webhook", Name: "ops", Status: target.StatusDelivered},
			}}},
		}

		s.recordNotificationMetrics()

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_notification_last_status{target="ops",type="webhook"} 0`)
	})
}

func TestScheduler_wakeScheduler(t *testing.T) {
	t.Parallel()
	t.Run("sends wake signal", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{rescheduleCh: make(chan struct{}, 1)}

		s.requestReschedule()

		assert.Len(t, s.rescheduleCh, 1)
	})

	t.Run("does not block when wake signal is already pending", func(t *testing.T) {
		t.Parallel()

		s := &Scheduler{rescheduleCh: make(chan struct{}, 1)}
		s.requestReschedule()

		s.requestReschedule()

		assert.Len(t, s.rescheduleCh, 1)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("records initial monitor metrics", func(t *testing.T) {
		t.Parallel()

		registry := metrics.NewRegistry()
		checkInMonitor := monitor.New("prometheus", time.Minute, time.Second, testLogger())

		_ = New(checkInMonitor, newRecordingNotifier(), registry, testLogger())

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="scheduled"} 1`)
	})

	t.Run("panics without monitor", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "check-in monitor must not be nil", func() {
			New(nil, newRecordingNotifier(), metrics.NewRegistry(), testLogger())
		})
	})

	t.Run("panics without notifier", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		require.PanicsWithValue(t, "scheduler notifier must not be nil", func() {
			New(checkInMonitor, nil, metrics.NewRegistry(), testLogger())
		})
	})

	t.Run("panics without metrics registry", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		require.PanicsWithValue(t, "scheduler metrics registry must not be nil", func() {
			New(checkInMonitor, newRecordingNotifier(), nil, testLogger())
		})
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		require.PanicsWithValue(t, "scheduler logger must not be nil", func() {
			New(checkInMonitor, newRecordingNotifier(), metrics.NewRegistry(), nil)
		})
	})
}

func TestEarlierTime(t *testing.T) {
	t.Parallel()
	t.Run("returns first when first is earlier", func(t *testing.T) {
		t.Parallel()

		first := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		second := first.Add(time.Second)

		got := earlierTime(first, second)

		assert.True(t, got.Equal(first))
	})

	t.Run("returns second when second is earlier", func(t *testing.T) {
		t.Parallel()

		second := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		first := second.Add(time.Second)

		got := earlierTime(first, second)

		assert.True(t, got.Equal(second))
	})
}

func TestNotificationRetryAfter(t *testing.T) {
	t.Parallel()
	t.Run("uses retry error duration", func(t *testing.T) {
		t.Parallel()

		err := retryAfterTestError{wait: 5 * time.Second}

		assert.Equal(t, 5*time.Second, notificationRetryAfter(err))
	})

	t.Run("uses default for zero retry duration", func(t *testing.T) {
		t.Parallel()

		err := retryAfterTestError{wait: 0}

		assert.Equal(t, defaultNotificationRetryAfter, notificationRetryAfter(err))
	})

	t.Run("uses default for generic error", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, defaultNotificationRetryAfter, notificationRetryAfter(errors.New("boom")))
	})
}

func TestNotificationStats(t *testing.T) {
	t.Parallel()
	t.Run("uses stats error values", func(t *testing.T) {
		t.Parallel()

		delivered, failed, pending := notificationStats(statsTestError{})

		assert.Equal(t, 2, delivered)
		assert.Equal(t, 1, failed)
		assert.Equal(t, 3, pending)
	})

	t.Run("uses default stats for generic error", func(t *testing.T) {
		t.Parallel()

		delivered, failed, pending := notificationStats(errors.New("boom"))

		assert.Equal(t, 0, delivered)
		assert.Equal(t, 1, failed)
		assert.Equal(t, 1, pending)
	})
}

func TestIncidentID(t *testing.T) {
	t.Parallel()
	t.Run("returns event incident id", func(t *testing.T) {
		t.Parallel()

		id := incidentID(monitor.Event{IncidentID: "incident"})

		assert.Equal(t, "incident", id)
	})

	t.Run("returns unknown for missing incident id", func(t *testing.T) {
		t.Parallel()

		id := incidentID(monitor.Event{})

		assert.Equal(t, "unknown", id)
	})
}

// scrapeMetrics renders the registry metrics response body.
func scrapeMetrics(t *testing.T, registry *metrics.Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	registry.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

type statusNotifier struct {
	status target.Status
}

// Notify implements target.Dispatcher for tests.
func (n statusNotifier) Notify(context.Context, monitor.Event) error {
	return nil
}

// NotificationStatus returns configured target status.
func (n statusNotifier) NotificationStatus() target.Status {
	return n.status
}

// schedulerTestEvent returns a scheduler notification event fixture.
func schedulerTestEvent(notificationID string) monitor.Event {
	return monitor.Event{
		IncidentID:     "incident",
		NotificationID: notificationID,
		CheckInName:    "default",
		LastCheckIn:    time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC),
		ExpectedBy:     time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC),
		OverdueSince:   time.Date(2026, 6, 19, 8, 1, 0, 0, time.UTC),
		AlertingAt:     time.Date(2026, 6, 19, 8, 1, 1, 0, time.UTC),
		Now:            time.Date(2026, 6, 19, 8, 1, 1, 0, time.UTC),
		Phase:          monitor.PhaseAlerting,
		Status:         monitor.StatusAlerting,
		Resolved:       false,
	}
}

// deliveryIDs returns notification IDs from pending deliveries.
func deliveryIDs(deliveries []pendingDelivery) []string {
	ids := make([]string, 0, len(deliveries))
	for _, target := range deliveries {
		ids = append(ids, target.event.NotificationID)
	}
	return ids
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type retryAfterTestError struct {
	wait time.Duration
}

// Error returns the test error message.
func (e retryAfterTestError) Error() string {
	return "retry"
}

// RetryAfter returns the test retry delay.
func (e retryAfterTestError) RetryAfter() time.Duration {
	return e.wait
}

type statsTestError struct{}

// Error returns the test error message.
func (statsTestError) Error() string {
	return "stats"
}

// NotificationStats returns test target stats.
func (statsTestError) NotificationStats() (delivered int, failed int, pending int) {
	return 2, 1, 3
}
