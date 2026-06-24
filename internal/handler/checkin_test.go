package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_CheckIn(t *testing.T) {
	t.Parallel()
	t.Run("accepts authorized post with compact response", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		var logs bytes.Buffer
		logger := testJSONLogger(&logs)
		api, checkInMonitor := testAPI("secret", logger)
		api.SetNowFn(func() time.Time { return now })

		req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
		req.RemoteAddr = "192.0.2.10:12345"
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()

		api.CheckIn().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.True(t, checkInMonitor.Snapshot().LastCheckIn.Equal(now))

		body := decodeJSONResponse(t, rec.Body.Bytes())
		assert.Equal(t, "ok", body["status"])
		assert.NotContains(t, body, "phase")
		assert.NotContains(t, body, "lastCheckIn")
		assert.NotContains(t, body, "expectedBy")

		entries := strings.Split(strings.TrimSpace(logs.String()), "\n")
		require.Len(t, entries, 1)
		assert.Contains(t, entries[0], `"msg":"first check-in received; check-in monitor active"`)
		assert.Contains(t, entries[0], `"remote":"192.0.2.10:12345"`)
		assert.Contains(t, entries[0], `"expectedEvery":"1m0s"`)
		assert.Contains(t, entries[0], `"alertingDelay":"1s"`)
	})

	t.Run("accepts authorized post with detailed response", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		api, _ := testAPI("secret", testLogger())
		api.SetNowFn(func() time.Time { return now })

		req := httptest.NewRequest(http.MethodPost, "/checkin?details=true", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()

		api.CheckIn().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		body := decodeJSONResponse(t, rec.Body.Bytes())
		assert.Equal(t, "ok", body["status"])
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

	t.Run("uses detailed response by default when configured", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		api, _ := testAPIWithOptions("", true, "dev", "none", testLogger())
		api.SetNowFn(func() time.Time { return now })
		rec := httptest.NewRecorder()

		api.CheckIn().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/checkin", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		body := decodeJSONResponse(t, rec.Body.Bytes())
		assert.Equal(t, "ok", body["status"])
		assert.Equal(t, "default", body["checkInName"])
		assert.Contains(t, body, "expectedEvery")
	})

	t.Run("rejects unauthorized request", func(t *testing.T) {
		t.Parallel()

		api, checkInMonitor := testAPI("secret", testLogger())
		rec := httptest.NewRecorder()

		api.CheckIn().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/checkin", nil))

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.JSONEq(t, `{"error":"unauthorized"}`, rec.Body.String())
		assert.Equal(t, monitor.PhaseScheduled, checkInMonitor.Snapshot().Phase)
	})
}
