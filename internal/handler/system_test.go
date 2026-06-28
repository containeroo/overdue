package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_Healthz(t *testing.T) {
	t.Parallel()
	t.Run("returns ok", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("", testLogger())
		rec := httptest.NewRecorder()

		api.Healthz().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Equal(t, "ok", rec.Body.String())
	})
}

func TestAPI_Readyz(t *testing.T) {
	t.Parallel()
	t.Run("returns ok", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("", testLogger())
		rec := httptest.NewRecorder()

		api.Readyz().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Equal(t, "ok", rec.Body.String())
	})
}

func TestAPI_Metrics(t *testing.T) {
	t.Parallel()
	t.Run("returns metrics", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("", testLogger())
		api.metrics.SetMonitorSnapshot("default", monitor.Snapshot{Phase: monitor.PhaseScheduled})
		rec := httptest.NewRecorder()

		api.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "overdue_monitor_phase")
	})
}

func TestAPI_Version(t *testing.T) {
	t.Parallel()
	t.Run("returns version information", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPIWithOptions("", false, "1.2.3", "abc123", testLogger())
		rec := httptest.NewRecorder()

		api.Version().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"version":"1.2.3","commit":"abc123"}`, rec.Body.String())
	})
}
