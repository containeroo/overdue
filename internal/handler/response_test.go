package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_respondJSON(t *testing.T) {
	t.Parallel()
	t.Run("writes json response", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("", testLogger())
		rec := httptest.NewRecorder()

		api.respondJSON(rec, http.StatusCreated, statusResponse{Status: "ok"})

		require.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	})

	t.Run("logs encoding failures", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		api, _ := testAPI("", slog.New(slog.NewJSONHandler(&logs, nil)))
		rec := httptest.NewRecorder()

		api.respondJSON(rec, http.StatusOK, map[string]any{"bad": func() {}})

		assert.Contains(t, logs.String(), "encode json response failed")
	})
}

func TestEncodeJSON(t *testing.T) {
	t.Parallel()
	t.Run("encodes value", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()

		err := encodeJSON(rec, http.StatusOK, statusResponse{Status: "ok"})

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	})

	t.Run("returns encoding error", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()

		err := encodeJSON(rec, http.StatusOK, map[string]any{"bad": func() {}})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "encode json")
	})
}

func TestEncodeText(t *testing.T) {
	t.Parallel()
	t.Run("encodes value", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()

		err := encodeText(rec, http.StatusOK, "ok")

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Equal(t, "ok", rec.Body.String())
	})
}

func TestNewCheckInResponse(t *testing.T) {
	t.Parallel()
	t.Run("returns ok status", func(t *testing.T) {
		t.Parallel()

		response := newCheckInResponse()

		assert.Equal(t, checkInResponseStatusOK, response.Status)
	})
}

func TestNewSnapshotResponse(t *testing.T) {
	t.Parallel()
	t.Run("omits zero timestamps", func(t *testing.T) {
		t.Parallel()

		body, err := json.Marshal(newSnapshotResponse(monitor.Snapshot{Phase: monitor.PhaseScheduled}))

		require.NoError(t, err)
		assert.JSONEq(t, `{"phase":"scheduled"}`, string(body))
	})

	t.Run("includes non-zero timestamps", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		body, err := json.Marshal(newSnapshotResponse(monitor.Snapshot{
			LastCheckIn:  now,
			ExpectedBy:   now.Add(time.Minute),
			OverdueSince: now.Add(time.Minute),
			AlertingAt:   now.Add(time.Minute + time.Second),
			Phase:        monitor.PhaseOverdue,
		}))

		require.NoError(t, err)
		assert.Contains(t, string(body), "lastCheckIn")
		assert.Contains(t, string(body), "expectedBy")
		assert.Contains(t, string(body), "overdueSince")
		assert.Contains(t, string(body), "alertingAt")
	})
}

func TestNewAcceptedCheckInDetailsResponse(t *testing.T) {
	t.Parallel()
	t.Run("adds ok status to detailed check-in response", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		response := newAcceptedCheckInDetailsResponse("prometheus", monitor.Snapshot{
			LastCheckIn: now,
			ExpectedBy:  now.Add(time.Minute),
			AlertingAt:  now.Add(time.Minute + 10*time.Second),
			Phase:       monitor.PhaseAwaiting,
		}, now)

		assert.Equal(t, checkInResponseStatusOK, response.Status)
		assert.Equal(t, "prometheus", response.CheckInName)
		assert.Equal(t, monitor.PhaseAwaiting, response.Phase)
	})
}

func TestNewCheckInDetailsResponse(t *testing.T) {
	t.Parallel()
	t.Run("builds response with timing details", func(t *testing.T) {
		t.Parallel()

		lastCheckIn := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		expectedBy := lastCheckIn.Add(time.Minute)
		overdueSince := expectedBy
		alertingAt := expectedBy.Add(10 * time.Second)
		now := alertingAt.Add(5 * time.Second)

		response := newCheckInDetailsResponse("prometheus", monitor.Snapshot{
			LastCheckIn:  lastCheckIn,
			ExpectedBy:   expectedBy,
			OverdueSince: overdueSince,
			AlertingAt:   alertingAt,
			Phase:        monitor.PhaseOverdue,
		}, now)

		assert.Empty(t, response.Status)
		assert.Equal(t, "prometheus", response.CheckInName)
		assert.Equal(t, monitor.PhaseOverdue, response.Phase)
		require.NotNil(t, response.LastCheckIn)
		require.NotNil(t, response.ExpectedBy)
		require.NotNil(t, response.OverdueSince)
		require.NotNil(t, response.AlertingAt)
		assert.True(t, response.LastCheckIn.Equal(lastCheckIn))
		assert.True(t, response.ExpectedBy.Equal(expectedBy))
		assert.True(t, response.OverdueSince.Equal(overdueSince))
		assert.True(t, response.AlertingAt.Equal(alertingAt))
		assert.Equal(t, "1m0s", response.ExpectedEvery)
		assert.Equal(t, "15s", response.OverdueFor)
		assert.Equal(t, "10s", response.AlertingDelay)
		assert.Equal(t, "1m10s", response.AlertingAfter)
		assert.Equal(t, "5s", response.AlertingFor)
	})

	t.Run("omits zero timing details", func(t *testing.T) {
		t.Parallel()

		response := newCheckInDetailsResponse("prometheus", monitor.Snapshot{
			Phase: monitor.PhaseScheduled,
		}, time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC))

		assert.Empty(t, response.Status)
		assert.Equal(t, "prometheus", response.CheckInName)
		assert.Equal(t, monitor.PhaseScheduled, response.Phase)
		assert.Nil(t, response.LastCheckIn)
		assert.Nil(t, response.ExpectedBy)
		assert.Nil(t, response.OverdueSince)
		assert.Nil(t, response.AlertingAt)
		assert.Empty(t, response.ExpectedEvery)
		assert.Empty(t, response.OverdueFor)
		assert.Empty(t, response.AlertingDelay)
		assert.Empty(t, response.AlertingAfter)
		assert.Empty(t, response.AlertingFor)
	})
}
