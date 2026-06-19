package render

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
)

func TestSampleAlertingEvent(t *testing.T) {
	t.Parallel()
	t.Run("returns complete alerting event", func(t *testing.T) {
		t.Parallel()

		event := SampleAlertingEvent()

		assert.Equal(t, "sample-incident", event.IncidentID)
		assert.Equal(t, "sample-alerting-notification", event.NotificationID)
		assert.Equal(t, "default", event.CheckInName)
		assert.Equal(t, monitor.PhaseAlerting, event.Phase)
		assert.Equal(t, monitor.StatusAlerting, event.Status)
		assert.False(t, event.Resolved)
		assert.True(t, event.ExpectedBy.Equal(event.LastCheckIn.Add(5*time.Second)))
		assert.True(t, event.AlertingAt.Equal(event.ExpectedBy.Add(3*time.Second)))
		assert.True(t, event.Now.Equal(event.AlertingAt))
	})
}

func TestSampleResolvedEvent(t *testing.T) {
	t.Parallel()
	t.Run("returns complete resolved event", func(t *testing.T) {
		t.Parallel()

		event := SampleResolvedEvent()

		assert.Equal(t, "sample-incident", event.IncidentID)
		assert.Equal(t, "sample-resolved-notification", event.NotificationID)
		assert.Equal(t, "default", event.CheckInName)
		assert.Equal(t, monitor.PhaseAwaiting, event.Phase)
		assert.Equal(t, monitor.StatusResolved, event.Status)
		assert.True(t, event.Resolved)
		assert.True(t, event.Now.After(event.AlertingAt))
	})
}
