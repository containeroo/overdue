package handler

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestAPI_logCheckInReceived(t *testing.T) {
	t.Parallel()
	t.Run("logs overdue restart", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		api := &API{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		api.logCheckInReceived(service.RecordCheckInResult{
			PreviousPhase: monitor.PhaseOverdue,
			Snapshot: monitor.Snapshot{
				LastCheckIn: now,
				ExpectedBy:  now.Add(time.Minute),
				AlertingAt:  now.Add(time.Minute + time.Second),
				Phase:       monitor.PhaseAwaiting,
			},
		}, now, RequestMetadata{})

		assert.Contains(t, logs.String(), "check-in received while overdue; next deadline scheduled")
		assert.Contains(t, logs.String(), monitor.PhaseOverdue)
	})

	t.Run("logs alerting restart", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		api := &API{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		api.logCheckInReceived(service.RecordCheckInResult{
			PreviousPhase: monitor.PhaseAlerting,
			Snapshot: monitor.Snapshot{
				LastCheckIn: now,
				ExpectedBy:  now.Add(time.Minute),
				AlertingAt:  now.Add(time.Minute + time.Second),
				Phase:       monitor.PhaseAwaiting,
			},
		}, now, RequestMetadata{})

		assert.Contains(t, logs.String(), "check-in received while alerting; next deadline scheduled")
		assert.Contains(t, logs.String(), monitor.PhaseAlerting)
	})

	t.Run("logs default restart", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		api := &API{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		api.logCheckInReceived(service.RecordCheckInResult{
			PreviousPhase: monitor.PhaseAwaiting,
			Snapshot: monitor.Snapshot{
				LastCheckIn: now,
				ExpectedBy:  now.Add(time.Minute),
				AlertingAt:  now.Add(time.Minute + time.Second),
				Phase:       monitor.PhaseAwaiting,
			},
		}, now, RequestMetadata{})

		assert.Contains(t, logs.String(), "check-in received; next deadline scheduled")
	})
}

func TestAPI_logSnapshotRequested(t *testing.T) {
	t.Parallel()
	t.Run("logs snapshot request", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		api := &API{logger: slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))}
		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		api.logSnapshotRequested(service.SnapshotResult{
			Snapshot: monitor.Snapshot{
				LastCheckIn: now,
				ExpectedBy:  now.Add(time.Minute),
				Phase:       monitor.PhaseAwaiting,
			},
		}, RequestMetadata{Remote: "192.0.2.10:12345"})

		assert.Contains(t, logs.String(), "snapshot requested")
		assert.Contains(t, logs.String(), "192.0.2.10:12345")
	})
}

func TestCheckInLogFields(t *testing.T) {
	t.Parallel()
	t.Run("builds full field set", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		fields := checkInLogFields(monitor.Snapshot{
			LastCheckIn: now,
			ExpectedBy:  now.Add(time.Minute),
			AlertingAt:  now.Add(time.Minute + time.Second),
			Phase:       monitor.PhaseAwaiting,
		}, now, RequestMetadata{Remote: "192.0.2.10:12345"})

		joined := strings.TrimSpace(strings.Join(anySliceToStrings(fields), " "))
		assert.Contains(t, joined, "remote")
		assert.Contains(t, joined, "192.0.2.10:12345")
		assert.Contains(t, joined, "expectedEvery")
		assert.Contains(t, joined, "alertingDelay")
	})

	t.Run("omits request fields when empty", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)

		fields := checkInLogFields(monitor.Snapshot{
			LastCheckIn: now,
			ExpectedBy:  now.Add(time.Minute),
			AlertingAt:  now.Add(time.Minute + time.Second),
			Phase:       monitor.PhaseAwaiting,
		}, now, RequestMetadata{})

		assert.NotContains(t, anySliceToStrings(fields), "remote")
	})
}

// anySliceToStrings converts fields to strings.
func anySliceToStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprint(value))
	}
	return out
}
