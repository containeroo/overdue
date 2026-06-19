package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/service"
	"github.com/stretchr/testify/require"
)

const (
	testExpectedEvery = time.Minute
	testAlertingDelay = time.Second
)

// testAPI builds an API fixture.
func testAPI(authToken string, logger *slog.Logger) (*API, *monitor.Monitor) {
	return testAPIWithOptions(authToken, false, "dev", "none", logger)
}

// testAPIWithOptions builds an API fixture with configurable API metadata.
func testAPIWithOptions(
	authToken string,
	responseDetails bool,
	version, commit string,
	logger *slog.Logger,
) (*API, *monitor.Monitor) {
	checkInMonitor := monitor.New("default", testExpectedEvery, testAlertingDelay, logger)
	registry := metrics.NewRegistry()
	service := service.NewCheckIn(checkInMonitor, registry)
	api := NewAPI(authToken, service, registry, responseDetails, version, commit, logger)
	return api, checkInMonitor
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testJSONLogger returns a JSON logger fixture.
func testJSONLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}

// decodeJSONResponse decodes a JSON response.
func decodeJSONResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var response map[string]any
	require.NoError(t, json.Unmarshal(body, &response))
	return response
}
