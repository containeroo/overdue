package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()
	t.Run("registers overdue metrics", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.SetMonitorSnapshot("prometheus", monitor.Snapshot{Phase: monitor.PhaseScheduled})
		registry.IncCheckInReceived("prometheus")
		rec := httptest.NewRecorder()

		registry.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "overdue_monitor_phase")
		assert.Contains(t, rec.Body.String(), "overdue_checkins_received_total")
	})
}

func TestRegistry_SetMonitorSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("updates monitor gauges", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		registry.SetMonitorSnapshot("prometheus", monitor.Snapshot{
			LastCheckIn: now,
			ExpectedBy:  now.Add(time.Minute),
			AlertingAt:  now.Add(time.Minute + time.Second),
			Phase:       monitor.PhaseAwaiting,
		})

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="scheduled"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="awaiting"} 1`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="overdue"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="alerting"} 0`)
		assert.Contains(t, body, `overdue_monitor_last_checkin_timestamp_seconds{check_in="prometheus"} 1.781856e+09`)
		assert.Contains(t, body, `overdue_monitor_expected_by_timestamp_seconds{check_in="prometheus"} 1.78185606e+09`)
		assert.Contains(t, body, `overdue_monitor_alerting_at_timestamp_seconds{check_in="prometheus"} 1.781856061e+09`)
	})

	t.Run("sets zero timestamp gauges for inactive monitor", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.SetMonitorSnapshot("prometheus", monitor.Snapshot{Phase: monitor.PhaseScheduled})

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="scheduled"} 1`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="awaiting"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="overdue"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="alerting"} 0`)
		assert.Contains(t, body, `overdue_monitor_last_checkin_timestamp_seconds{check_in="prometheus"} 0`)
		assert.Contains(t, body, `overdue_monitor_expected_by_timestamp_seconds{check_in="prometheus"} 0`)
		assert.Contains(t, body, `overdue_monitor_alerting_at_timestamp_seconds{check_in="prometheus"} 0`)
	})
}

func TestRegistry_IncCheckInReceived(t *testing.T) {
	t.Parallel()
	t.Run("increments check-in counter", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		registry.IncCheckInReceived("prometheus")
		registry.IncCheckInReceived("prometheus")

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_checkins_received_total{check_in="prometheus"} 2`)
	})
}

func TestRegistry_NotificationMetrics(t *testing.T) {
	t.Parallel()
	t.Run("increments notification counters", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		registry.IncNotificationQueued("prometheus", monitor.StatusAlerting)
		registry.IncNotificationSkipped("prometheus", monitor.StatusResolved, "no_resolved_receivers")
		registry.IncNotificationQueueFailed("prometheus", monitor.StatusAlerting)

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_notifications_queued_total{check_in="prometheus",status="alerting"} 1`)
		assert.Contains(t, body, `overdue_notifications_skipped_total{check_in="prometheus",reason="no_resolved_receivers",status="resolved"} 1`)
		assert.Contains(t, body, `overdue_notifications_queue_failed_total{check_in="prometheus",status="alerting"} 1`)
	})
}

func TestRegistry_setActiveMonitorPhase(t *testing.T) {
	t.Parallel()
	t.Run("sets only current phase active", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		registry.setActiveMonitorPhase("prometheus", monitor.PhaseAlerting)

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="scheduled"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="awaiting"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="overdue"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="alerting"} 1`)
	})

	t.Run("sets all known phases inactive for unknown phase", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		registry.setActiveMonitorPhase("prometheus", monitor.Phase("unknown"))

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="scheduled"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="awaiting"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="overdue"} 0`)
		assert.Contains(t, body, `overdue_monitor_phase{check_in="prometheus",phase="alerting"} 0`)
	})
}

func TestTimestampValue(t *testing.T) {
	t.Parallel()
	t.Run("returns zero for zero time", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, float64(0), timestampValue(time.Time{}))
	})

	t.Run("returns unix timestamp seconds", func(t *testing.T) {
		t.Parallel()

		value := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		assert.Equal(t, float64(1781856000), timestampValue(value))
	})
}

// scrapeMetrics renders the registry metrics response body.
func scrapeMetrics(t *testing.T, registry *Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	registry.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
