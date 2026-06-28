package scheduler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	overduenotify "github.com/containeroo/overdue/internal/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contextValueKey string

type recordingNotifier struct {
	mu            sync.Mutex
	events        []monitor.Event
	receivers     [][]kit.ReceiverID
	contextValues []any
	called        chan monitor.Event
}

func newRecordingNotifier() *recordingNotifier {
	return &recordingNotifier{called: make(chan monitor.Event, 10)}
}

func (n *recordingNotifier) Enqueue(ctx context.Context, notification kit.Notification) (string, error) {
	event, ok := notification.(*overduenotify.Event)
	if !ok {
		return notification.ID(), nil
	}

	var receivers []kit.ReceiverID
	if router, ok := notification.(kit.ReceiverRouter); ok {
		receivers = router.ReceiverIDs()
	}

	n.mu.Lock()
	n.events = append(n.events, event.MonitorEvent)
	n.receivers = append(n.receivers, receivers)
	n.contextValues = append(n.contextValues, ctx.Value(contextValueKey("request")))
	n.mu.Unlock()
	n.called <- event.MonitorEvent
	return notification.ID(), nil
}

func (n *recordingNotifier) Events() []monitor.Event {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]monitor.Event(nil), n.events...)
}

func (n *recordingNotifier) Receivers() [][]kit.ReceiverID {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([][]kit.ReceiverID, len(n.receivers))
	for i, receivers := range n.receivers {
		out[i] = append([]kit.ReceiverID(nil), receivers...)
	}
	return out
}

func (n *recordingNotifier) ContextValues() []any {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]any(nil), n.contextValues...)
}

func TestScheduler_CheckInName(t *testing.T) {
	t.Parallel()

	checkInMonitor := monitor.New("prometheus", time.Minute, time.Second, testLogger())
	s := New(checkInMonitor, newRecordingNotifier(), nil, metrics.NewRegistry(), testLogger())

	assert.Equal(t, "prometheus", s.CheckInName())
}

func TestScheduler_RecordCheckIn(t *testing.T) {
	t.Parallel()

	t.Run("records check-in and wakes scheduler", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		s := New(checkInMonitor, newRecordingNotifier(), nil, metrics.NewRegistry(), testLogger())
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		result := s.RecordCheckIn(now)

		assert.Equal(t, monitor.PhaseAwaiting, result.Snapshot.Phase)
		select {
		case <-s.rescheduleCh:
		default:
			require.Fail(t, "scheduler was not woken")
		}
	})

	t.Run("enqueues resolved notification for configured resolved receivers", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
		notifier := newRecordingNotifier()
		s := New(checkInMonitor, notifier, overduenotify.NewRouter([]kit.ReceiverID{"ops"}), metrics.NewRegistry(), testLogger())

		s.RecordCheckIn(start)
		s.Check(context.Background(), start.Add(time.Minute))
		ctx := context.WithValue(context.Background(), contextValueKey("request"), "check-in-request")
		result := s.RecordCheckInContext(ctx, start.Add(2*time.Minute))

		require.True(t, result.ShouldNotify)
		assert.Equal(t, monitor.StatusResolved, result.Event.Status)
		events := notifier.Events()
		require.Len(t, events, 2)
		assert.Equal(t, monitor.StatusResolved, events[1].Status)
		assert.Equal(t, []kit.ReceiverID{"ops"}, notifier.Receivers()[1])
		assert.Equal(t, "check-in-request", notifier.ContextValues()[1])
	})

	t.Run("skips resolved notification without configured resolved receivers", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
		notifier := newRecordingNotifier()
		s := New(checkInMonitor, notifier, nil, metrics.NewRegistry(), testLogger())

		s.RecordCheckIn(start)
		s.Check(context.Background(), start.Add(time.Minute))
		ctx := context.WithValue(context.Background(), contextValueKey("request"), "check-in-request")
		result := s.RecordCheckInContext(ctx, start.Add(2*time.Minute))

		require.True(t, result.ShouldNotify)
		events := notifier.Events()
		require.Len(t, events, 1)
		assert.Equal(t, monitor.StatusAlerting, events[0].Status)
	})
}

func TestScheduler_Snapshot(t *testing.T) {
	t.Parallel()

	checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
	s := New(checkInMonitor, newRecordingNotifier(), nil, metrics.NewRegistry(), testLogger())
	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	checkInMonitor.RecordCheckIn(now)

	snapshot := s.Snapshot()

	assert.Equal(t, monitor.PhaseAwaiting, snapshot.Phase)
	assert.True(t, snapshot.LastCheckIn.Equal(now))
}

func TestScheduler_Check(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	notifier := newRecordingNotifier()
	checkInMonitor := monitor.New("default", time.Minute, 0, testLogger())
	checkInMonitor.RecordCheckIn(start)
	s := New(checkInMonitor, notifier, nil, metrics.NewRegistry(), testLogger())

	s.Check(context.Background(), start.Add(time.Minute))

	events := notifier.Events()
	require.Len(t, events, 1)
	assert.Equal(t, monitor.StatusAlerting, events[0].Status)
	assert.Equal(t, monitor.PhaseAlerting, s.Snapshot().Phase)
}

func TestScheduler_Run(t *testing.T) {
	t.Parallel()

	t.Run("uses expected deadline before overdue deadline", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		notifier := newRecordingNotifier()
		checkInMonitor := monitor.New("default", 30*time.Millisecond, 80*time.Millisecond, testLogger())
		s := New(checkInMonitor, notifier, nil, metrics.NewRegistry(), testLogger())
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
		s := New(checkInMonitor, notifier, nil, metrics.NewRegistry(), testLogger())
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

func TestScheduler_enqueue(t *testing.T) {
	t.Parallel()

	t.Run("queues alerting notifications to all receivers", func(t *testing.T) {
		t.Parallel()

		notifier := newRecordingNotifier()
		s := &Scheduler{notifier: notifier, router: overduenotify.NewRouter(nil), logger: testLogger()}
		event := schedulerTestEvent("notification")

		s.enqueue(context.Background(), event)

		events := notifier.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "notification", events[0].NotificationID)
		assert.Nil(t, notifier.Receivers()[0])
	})

	t.Run("skips resolved notifications without receivers", func(t *testing.T) {
		t.Parallel()

		notifier := newRecordingNotifier()
		s := &Scheduler{notifier: notifier, router: overduenotify.NewRouter(nil), logger: testLogger()}
		event := schedulerTestEvent("notification")
		event.Resolved = true
		event.Status = monitor.StatusResolved

		s.enqueue(context.Background(), event)

		assert.Empty(t, notifier.Events())
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

		_ = New(checkInMonitor, newRecordingNotifier(), nil, registry, testLogger())

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="scheduled"} 1`)
	})

	t.Run("panics without monitor", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "check-in monitor must not be nil", func() {
			New(nil, newRecordingNotifier(), nil, metrics.NewRegistry(), testLogger())
		})
	})

	t.Run("panics without notifier", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		require.PanicsWithValue(t, "scheduler notifier must not be nil", func() {
			New(checkInMonitor, nil, nil, metrics.NewRegistry(), testLogger())
		})
	})

	t.Run("panics without metrics registry", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		require.PanicsWithValue(t, "scheduler metrics registry must not be nil", func() {
			New(checkInMonitor, newRecordingNotifier(), nil, nil, testLogger())
		})
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		require.PanicsWithValue(t, "scheduler logger must not be nil", func() {
			New(checkInMonitor, newRecordingNotifier(), nil, metrics.NewRegistry(), nil)
		})
	})
}

func scrapeMetrics(t *testing.T, registry *metrics.Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	registry.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

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
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
