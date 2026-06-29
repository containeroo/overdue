package service

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCheckIn(t *testing.T) {
	t.Parallel()
	t.Run("creates service", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := newRecordingCheckInMonitor("default")

		service := NewCheckIn(checkInMonitor, metrics.NewRegistry())

		require.NotNil(t, service)
	})

	t.Run("panics without monitor", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "check-in monitor must not be nil", func() {
			NewCheckIn(nil, metrics.NewRegistry())
		})
	})

	t.Run("panics without metrics registry", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := newRecordingCheckInMonitor("default")

		require.PanicsWithValue(t, "check-in metrics registry must not be nil", func() {
			NewCheckIn(checkInMonitor, nil)
		})
	})
}

func TestService_CheckInName(t *testing.T) {
	t.Parallel()
	t.Run("returns configured check-in name", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := newRecordingCheckInMonitor("prometheus")
		service := NewCheckIn(checkInMonitor, metrics.NewRegistry())

		assert.Equal(t, "prometheus", service.CheckInName())
	})
}

func TestService_RecordCheckIn(t *testing.T) {
	t.Parallel()
	t.Run("records check-in", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := newRecordingCheckInMonitor("prometheus")
		service := NewCheckIn(checkInMonitor, metrics.NewRegistry())

		result := service.RecordCheckIn(context.Background(), now)

		assert.Equal(t, "prometheus", result.CheckInName)
		assert.Equal(t, monitor.PhaseAwaiting, result.Snapshot.Phase)
		assert.Equal(t, monitor.PhaseScheduled, result.PreviousPhase)
	})

	t.Run("passes context to monitor", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := newRecordingCheckInMonitor("prometheus")
		service := NewCheckIn(checkInMonitor, metrics.NewRegistry())
		ctx := context.WithValue(context.Background(), contextValueKey("request"), "check-in-request")

		service.RecordCheckIn(ctx, now)

		assert.Equal(t, "check-in-request", checkInMonitor.contextValue)
	})

	t.Run("records check-in metrics", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		registry := metrics.NewRegistry()
		checkInMonitor := newRecordingCheckInMonitor("prometheus")
		service := NewCheckIn(checkInMonitor, registry)

		service.RecordCheckIn(context.Background(), now)

		body := scrapeMetrics(t, registry)
		assert.Contains(t, body, `overdue_checkins_received_total{check_in="prometheus"} 1`)
	})
}

func TestService_Snapshot(t *testing.T) {
	t.Parallel()
	t.Run("returns monitor snapshot", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := newRecordingCheckInMonitor("prometheus")
		service := NewCheckIn(checkInMonitor, metrics.NewRegistry())
		checkInMonitor.RecordCheckIn(now)

		snapshot := service.Snapshot()

		assert.Equal(t, "prometheus", snapshot.CheckInName)
		assert.Equal(t, monitor.PhaseAwaiting, snapshot.Snapshot.Phase)
		assert.True(t, snapshot.Snapshot.LastCheckIn.Equal(now))
	})
}

type contextValueKey string

type recordingCheckInMonitor struct {
	*monitor.Monitor
	contextValue any
}

func newRecordingCheckInMonitor(name string) *recordingCheckInMonitor {
	return &recordingCheckInMonitor{
		Monitor: monitor.New(name, time.Minute, time.Second, testLogger()),
	}
}

func (m *recordingCheckInMonitor) RecordCheckInContext(ctx context.Context, at time.Time) monitor.RecordResult {
	m.contextValue = ctx.Value(contextValueKey("request"))
	return m.Monitor.RecordCheckIn(at)
}

// scrapeMetrics renders the registry metrics response body.
func scrapeMetrics(t *testing.T, registry *metrics.Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	registry.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
