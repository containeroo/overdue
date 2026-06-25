package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/containeroo/overdue/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_Status(t *testing.T) {
	t.Parallel()
	t.Run("returns status snapshot", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		api, checkInMonitor := testAPI("", testLogger())
		checkInMonitor.RecordCheckIn(now)
		rec := httptest.NewRecorder()

		api.Status().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"phase":"awaiting"`)
		assert.Contains(t, rec.Body.String(), `"lastCheckIn"`)
	})

	t.Run("returns detailed status", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		api, checkInMonitor := testAPI("", testLogger())
		checkInMonitor.RecordCheckIn(now)
		rec := httptest.NewRecorder()

		api.Status().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status?details=true", nil))

		require.Equal(t, http.StatusOK, rec.Code)

		body := decodeJSONResponse(t, rec.Body.Bytes())
		assert.NotContains(t, body, "status")
		assert.Equal(t, "default", body["checkInName"])
		assert.Equal(t, string(monitor.PhaseAwaiting), body["phase"])
		assert.Equal(t, "1m0s", body["expectedEvery"])
		assert.Equal(t, "1s", body["alertingDelay"])
		assert.Equal(t, "1m1s", body["alertingAfter"])
		assert.Contains(t, body, "lastCheckIn")
		assert.Contains(t, body, "expectedBy")
		assert.Contains(t, body, "alertingAt")
		assert.NotContains(t, body, "overdueSince")
	})

	t.Run("returns notification status in detailed status", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		logger := testLogger()
		checkInMonitor := &statusMonitor{
			Monitor: monitor.New("default", testExpectedEvery, testAlertingDelay, logger),
			status: target.Status{
				Status:    target.StatusPartialFailure,
				Total:     2,
				Delivered: 1,
				Failed:    1,
				Pending:   1,
				Targets: []target.TargetStatus{
					{
						Type:            "webhook",
						Name:            "teams",
						Status:          target.StatusDelivered,
						LastAttemptAt:   &now,
						LastDeliveredAt: &now,
					},
					{
						Type:          "email",
						Name:          "email",
						Status:        target.StatusFailed,
						LastAttemptAt: &now,
					},
				},
			},
		}
		checkInMonitor.RecordCheckIn(now)
		registry := metrics.NewRegistry()
		api := NewAPI("", service.NewCheckIn(checkInMonitor, registry), registry, false, "dev", "none", logger)
		rec := httptest.NewRecorder()

		api.Status().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status?details=true", nil))

		require.Equal(t, http.StatusOK, rec.Code)

		body := decodeJSONResponse(t, rec.Body.Bytes())
		notifications, ok := body["notifications"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, string(target.StatusPartialFailure), notifications["status"])
		assert.Equal(t, float64(2), notifications["total"])
		assert.Equal(t, float64(1), notifications["delivered"])
		assert.Equal(t, float64(1), notifications["failed"])
		assert.Equal(t, float64(1), notifications["pending"])
		assert.NotContains(t, notifications, "error")

		targets, ok := notifications["targets"].([]any)
		require.True(t, ok)
		require.Len(t, targets, 2)
		first, ok := targets[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "webhook", first["type"])
		assert.Equal(t, "teams", first["name"])
		assert.Equal(t, string(target.StatusDelivered), first["status"])
		assert.Contains(t, first, "lastAttemptAt")
		assert.Contains(t, first, "lastDeliveredAt")
		assert.NotContains(t, first, "error")

		second, ok := targets[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "email", second["type"])
		assert.Equal(t, "email", second["name"])
		assert.Equal(t, string(target.StatusFailed), second["status"])
		assert.Contains(t, second, "lastAttemptAt")
		assert.NotContains(t, second, "lastDeliveredAt")
		assert.NotContains(t, second, "error")
	})

	t.Run("rejects unauthorized status", func(t *testing.T) {
		t.Parallel()

		api, checkInMonitor := testAPI("secret", testLogger())
		rec := httptest.NewRecorder()

		api.Status().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.JSONEq(t, `{"error":"unauthorized"}`, rec.Body.String())
		assert.Equal(t, monitor.PhaseScheduled, checkInMonitor.Snapshot().Phase)
	})
}

type statusMonitor struct {
	*monitor.Monitor
	status target.Status
}

func (m *statusMonitor) NotificationStatus() target.Status {
	return m.status
}
