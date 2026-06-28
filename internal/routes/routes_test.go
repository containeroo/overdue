package routes

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/handler"
	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Parallel()
	t.Run("mounts configured check-in route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/heartbeat", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/overdue/heartbeat", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	})

	t.Run("rejects get check-in route by default", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/checkin", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})

	t.Run("mounts get check-in route when enabled", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", true, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/checkin", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	})

	t.Run("does not mount unconfigured routes", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/heartbeat", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/overdue/old-route", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("does not serve root web ui", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/heartbeat", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("does not mount removed history route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/history", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("mounts metrics route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/metrics", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "overdue_monitor_phase")
	})

	t.Run("mounts status route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/status", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"phase":"scheduled"`)
	})

	t.Run("mounts version route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/version", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"version":"dev","commit":"none"}`, rec.Body.String())
	})

	t.Run("mounts healthz route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/healthz", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok", rec.Body.String())
	})

	t.Run("mounts post healthz route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/overdue/healthz", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok", rec.Body.String())
	})

	t.Run("rejects unauthorized status route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "secret")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/overdue/status", nil))

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error":"unauthorized"`)
	})

	t.Run("accepts authorized check-in route", func(t *testing.T) {
		t.Parallel()

		h := testRouter("/checkin", "/overdue", false, "secret")
		req := httptest.NewRequest(http.MethodPost, "/overdue/checkin", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	})
}

// testRouter builds a router fixture.
func testRouter(checkInPath, routePrefix string, allowGETCheckIn bool, authToken string) http.Handler {
	logger := testLogger()
	checkInMonitor := monitor.New("default", time.Minute, time.Second, logger)
	registry := metrics.NewRegistry()
	registry.SetMonitorSnapshot(checkInMonitor.CheckInName(), checkInMonitor.Snapshot())
	svc := service.NewCheckIn(checkInMonitor, registry)
	api := handler.NewAPI(authToken, svc, registry, false, "dev", "none", logger)

	return NewRouter(checkInPath, routePrefix, allowGETCheckIn, api)
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
