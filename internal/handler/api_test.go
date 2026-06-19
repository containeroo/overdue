package handler

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPI(t *testing.T) {
	t.Parallel()
	t.Run("creates api", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("secret", testLogger())

		require.NotNil(t, api)
		assert.Equal(t, "secret", api.authToken)
		assert.NotNil(t, api.service)
		assert.NotNil(t, api.metrics)
		assert.False(t, api.responseDetails)
		assert.Equal(t, "dev", api.version)
		assert.Equal(t, "none", api.commit)
	})

	t.Run("panics without service", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "check-in service must not be nil", func() {
			NewAPI("", nil, metrics.NewRegistry(), false, "dev", "none", testLogger())
		})
	})

	t.Run("panics without metrics registry", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		svc := service.NewCheckIn(checkInMonitor, metrics.NewRegistry())

		require.PanicsWithValue(t, "metrics registry must not be nil", func() {
			NewAPI("", svc, nil, false, "dev", "none", testLogger())
		})
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := monitor.New("default", time.Minute, time.Second, testLogger())
		registry := metrics.NewRegistry()
		svc := service.NewCheckIn(checkInMonitor, registry)

		require.PanicsWithValue(t, "api logger must not be nil", func() {
			NewAPI("", svc, registry, false, "dev", "none", nil)
		})
	})

	t.Run("stores constructor config", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPIWithOptions("secret", true, "1.2.3", "abc123", testLogger())

		assert.Equal(t, "secret", api.authToken)
		assert.True(t, api.responseDetails)
		assert.Equal(t, "1.2.3", api.version)
		assert.Equal(t, "abc123", api.commit)
	})
}

func TestAPI_SetNowFn(t *testing.T) {
	t.Parallel()
	t.Run("sets clock", func(t *testing.T) {
		t.Parallel()

		want := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		api, _ := testAPI("", testLogger())

		api.SetNowFn(func() time.Time { return want })

		assert.True(t, api.nowFn().Equal(want))
	})

	t.Run("panics without clock", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("", testLogger())

		require.PanicsWithValue(t, "api clock must not be nil", func() {
			api.SetNowFn(nil)
		})
	})
}
