package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
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
