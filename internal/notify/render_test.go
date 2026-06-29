package notify

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
)

// TestNewData tests template data construction.
func TestNewData(t *testing.T) {
	t.Parallel()

	t.Run("builds template data", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
		vars := varsFromConfig(AppData{Version: "dev"}, map[string]string{"channel": "alerts"})
		data := NewData(monitor.Event{
			IncidentID:     "incident-1",
			NotificationID: "notification-1",
			CheckInName:    "api",
			Now:            now,
			Status:         monitor.StatusAlerting,
		}, "ops", vars, "subject")

		assert.Equal(t, "incident-1", data.IncidentID)
		assert.Equal(t, "notification-1", data.NotificationID)
		assert.Equal(t, "api", data.CheckInName)
		assert.Equal(t, "subject", data.Title)
		assert.Equal(t, "subject", data.Subject)
		assert.Equal(t, "ops", data.Receiver)
		assert.Equal(t, map[string]any{"channel": "alerts"}, data.Vars)
		assert.Equal(t, map[string]string{"channel": "alerts"}, data.CustomData)
		assert.Equal(t, "dev", data.App.Version)
	})
}

// TestText tests default event summaries.
func TestText(t *testing.T) {
	t.Parallel()

	t.Run("uses resolved text", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `Check-in "api" is resolved:`, text(monitor.Event{CheckInName: "api", Resolved: true}))
	})

	t.Run("uses overdue text", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `Check-in "api" is overdue:`, text(monitor.Event{CheckInName: "api"}))
	})
}

// TestVarsFromConfig tests receiver variable construction.
func TestVarsFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("contains app without custom data", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]any{appVar: AppData{Version: "dev"}}, varsFromConfig(AppData{Version: "dev"}, nil))
	})

	t.Run("adds custom data as direct vars and CustomData", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]any{
			appVar:        AppData{Version: "dev"},
			customDataVar: map[string]string{"channel": "alerts"},
			"channel":     "alerts",
		}, varsFromConfig(AppData{Version: "dev"}, map[string]string{"channel": "alerts"}))
	})
}

// TestAppData tests App extraction from receiver variables.
func TestAppData(t *testing.T) {
	t.Parallel()

	t.Run("returns zero value without vars", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, AppData{}, appData(nil))
	})

	t.Run("returns configured app", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, AppData{Version: "dev"}, appData(map[string]any{appVar: AppData{Version: "dev"}}))
	})
}

// TestCustomData tests CustomData extraction from receiver variables.
func TestCustomData(t *testing.T) {
	t.Parallel()

	t.Run("returns nil without vars", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, customData(nil))
	})

	t.Run("prefers configured CustomData map", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]string{"channel": "alerts"}, customData(map[string]any{customDataVar: map[string]string{"channel": "alerts"}, "ignored": 3}))
	})

	t.Run("falls back to string vars", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]string{"channel": "alerts"}, customData(map[string]any{"channel": "alerts", "retries": 3}))
	})
}

// TestPublicVars tests public receiver variable extraction.
func TestPublicVars(t *testing.T) {
	t.Parallel()

	t.Run("returns nil without vars", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, publicVars(nil))
	})

	t.Run("excludes app and custom data container", func(t *testing.T) {
		t.Parallel()

		vars := map[string]any{appVar: AppData{Version: "dev"}, customDataVar: map[string]string{"channel": "alerts"}, "channel": "alerts"}
		got := publicVars(vars)
		vars["channel"] = "changed"

		assert.Equal(t, map[string]any{"channel": "alerts"}, got)
	})
}

// TestCloneStringMap tests string map copying.
func TestCloneStringMap(t *testing.T) {
	t.Parallel()

	t.Run("returns nil without values", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, cloneStringMap(nil))
	})

	t.Run("copies values", func(t *testing.T) {
		t.Parallel()

		values := map[string]string{"channel": "alerts"}
		got := cloneStringMap(values)
		values["channel"] = "changed"

		assert.Equal(t, map[string]string{"channel": "alerts"}, got)
	})
}
