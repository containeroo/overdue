package monitor

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "monitor logger must not be nil", func() {
			New("default", time.Minute, time.Second, nil)
		})
	})

	t.Run("uses configured check-in name", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("prometheus", time.Minute, time.Second, testLogger())

		assert.Equal(t, "prometheus", checkInMonitor.CheckInName())
		assert.Equal(t, PhaseScheduled, checkInMonitor.Snapshot().Phase)
	})

	t.Run("trims configured check-in name", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New(" prometheus ", time.Minute, time.Second, testLogger())

		assert.Equal(t, "prometheus", checkInMonitor.CheckInName())
	})

	t.Run("defaults blank check-in name", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New(" ", time.Minute, time.Second, testLogger())

		assert.Equal(t, "default", checkInMonitor.CheckInName())
	})
}

func TestCheckInName(t *testing.T) {
	t.Parallel()

	t.Run("returns configured check-in name", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("prometheus", time.Minute, time.Second, testLogger())

		assert.Equal(t, "prometheus", checkInMonitor.CheckInName())
	})
}

func TestRecordCheckIn(t *testing.T) {
	t.Parallel()

	t.Run("arms monitor on first check-in", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())

		record := checkInMonitor.RecordCheckIn(start)
		snapshot := record.Snapshot

		assert.False(t, record.ShouldNotify)
		assert.Equal(t, PhaseScheduled, record.PreviousPhase)
		assert.Equal(t, Event{}, record.Event)
		assert.Equal(t, PhaseAwaiting, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.Equal(start))
		assert.True(t, snapshot.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, snapshot.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
	})

	t.Run("restarts expected deadline while overdue", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.Check(start.Add(time.Minute))

		record := checkInMonitor.RecordCheckIn(start.Add(time.Minute + 5*time.Second))
		snapshot := record.Snapshot

		assert.False(t, record.ShouldNotify)
		assert.Equal(t, PhaseOverdue, record.PreviousPhase)
		assert.Equal(t, Event{}, record.Event)
		assert.Equal(t, PhaseAwaiting, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.Equal(start.Add(time.Minute+5*time.Second)))
		assert.True(t, snapshot.ExpectedBy.Equal(start.Add(2*time.Minute+5*time.Second)))
		assert.True(t, snapshot.AlertingAt.Equal(start.Add(2*time.Minute+15*time.Second)))
	})

	t.Run("emits resolved event after alerting", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		resolvedAt := start.Add(2 * time.Minute)
		checkInMonitor := New("default", time.Minute, 0, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1", "notification-resolved-1")
		checkInMonitor.RecordCheckIn(start)

		alerting := checkInMonitor.Check(start.Add(time.Minute))
		require.True(t, alerting.ShouldNotify)

		resolved := checkInMonitor.RecordCheckIn(resolvedAt)

		require.True(t, resolved.ShouldNotify)
		assert.Equal(t, PhaseAlerting, resolved.PreviousPhase)
		assert.Equal(t, "incident-1", resolved.Event.IncidentID)
		assert.Equal(t, "notification-resolved-1", resolved.Event.NotificationID)
		assert.Equal(t, "default", resolved.Event.CheckInName)
		assert.True(t, resolved.Event.LastCheckIn.Equal(start))
		assert.True(t, resolved.Event.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, resolved.Event.AlertingAt.Equal(start.Add(time.Minute)))
		assert.True(t, resolved.Event.Now.Equal(resolvedAt))
		assert.Equal(t, PhaseAwaiting, resolved.Event.Phase)
		assert.Equal(t, StatusResolved, resolved.Event.Status)
		assert.True(t, resolved.Event.Resolved)
		assert.Equal(t, PhaseAwaiting, resolved.Snapshot.Phase)
	})
}

func TestSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("returns scheduled before first check-in", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		snapshot := checkInMonitor.Snapshot()

		assert.Equal(t, PhaseScheduled, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.IsZero())
		assert.True(t, snapshot.ExpectedBy.IsZero())
		assert.True(t, snapshot.AlertingAt.IsZero())
	})

	t.Run("returns active schedule after check-in", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		snapshot := checkInMonitor.Snapshot()

		assert.Equal(t, PhaseAwaiting, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.Equal(start))
		assert.True(t, snapshot.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, snapshot.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
	})

	t.Run("returns overdue state after expected deadline", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.Check(start.Add(time.Minute))

		snapshot := checkInMonitor.Snapshot()

		assert.Equal(t, PhaseOverdue, snapshot.Phase)
		assert.True(t, snapshot.OverdueSince.Equal(start.Add(time.Minute)))
		assert.True(t, snapshot.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
	})
}

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("does nothing before first check-in", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())

		result := checkInMonitor.Check(start.Add(time.Hour))

		assert.False(t, result.ShouldNotify)
		assert.Equal(t, Event{}, result.Event)
		assert.Equal(t, PhaseScheduled, checkInMonitor.Snapshot().Phase)
	})

	t.Run("emits alerting event after alerting delay", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1")
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.Check(start.Add(time.Minute))

		result := checkInMonitor.Check(start.Add(time.Minute + 10*time.Second))

		require.True(t, result.ShouldNotify)
		assert.Equal(t, "incident-1", result.Event.IncidentID)
		assert.Equal(t, "notification-alerting-1", result.Event.NotificationID)
		assert.Equal(t, StatusAlerting, result.Event.Status)
		assert.Empty(t, result.Event.Title)
		assert.Empty(t, result.Event.Text)
		assert.True(t, result.Event.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, result.Event.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
		assert.Equal(t, PhaseAlerting, checkInMonitor.Snapshot().Phase)
	})

	t.Run("does not repeat alerting event before a new check-in", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 0, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1", "notification-resolved-1")
		checkInMonitor.RecordCheckIn(start)

		first := checkInMonitor.Check(start.Add(time.Minute))
		second := checkInMonitor.Check(start.Add(time.Minute))
		third := checkInMonitor.Check(start.Add(time.Minute + time.Second))

		assert.True(t, first.ShouldNotify)
		assert.False(t, second.ShouldNotify)
		assert.Equal(t, Event{}, second.Event)
		assert.False(t, third.ShouldNotify)
		assert.Equal(t, Event{}, third.Event)
	})
}

func TestAdvance(t *testing.T) {
	t.Parallel()

	t.Run("returns empty result while inactive", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())

		event, shouldNotify := checkInMonitor.advance(now)

		assert.False(t, shouldNotify)
		assert.Equal(t, Event{}, event)
		assert.Equal(t, PhaseScheduled, checkInMonitor.Snapshot().Phase)
	})

	t.Run("keeps awaiting before expected deadline", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		event, shouldNotify := checkInMonitor.advance(start.Add(30 * time.Second))

		assert.False(t, shouldNotify)
		assert.Equal(t, Event{}, event)
		assert.Equal(t, PhaseAwaiting, checkInMonitor.Snapshot().Phase)
	})

	t.Run("enters overdue at expected deadline without notifying", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		event, shouldNotify := checkInMonitor.advance(start.Add(time.Minute))

		assert.False(t, shouldNotify)
		assert.Equal(t, Event{}, event)
		assert.Equal(t, PhaseOverdue, checkInMonitor.Snapshot().Phase)
	})

	t.Run("enters alerting immediately when alerting deadline is reached", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 0, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1")
		checkInMonitor.RecordCheckIn(start)

		event, shouldNotify := checkInMonitor.advance(start.Add(time.Minute))

		require.True(t, shouldNotify)
		assert.Equal(t, "incident-1", event.IncidentID)
		assert.Equal(t, "notification-alerting-1", event.NotificationID)
		assert.Equal(t, StatusAlerting, event.Status)
		assert.Equal(t, PhaseAlerting, checkInMonitor.Snapshot().Phase)
	})

	t.Run("enters alerting from overdue at alerting deadline", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1")
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.advance(start.Add(time.Minute))

		event, shouldNotify := checkInMonitor.advance(start.Add(time.Minute + 10*time.Second))

		require.True(t, shouldNotify)
		assert.Equal(t, "incident-1", event.IncidentID)
		assert.Equal(t, "notification-alerting-1", event.NotificationID)
		assert.Equal(t, PhaseAlerting, checkInMonitor.Snapshot().Phase)
	})

	t.Run("does nothing while already alerting", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 0, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1")
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.advance(start.Add(time.Minute))

		event, shouldNotify := checkInMonitor.advance(start.Add(time.Minute + time.Second))

		assert.False(t, shouldNotify)
		assert.Equal(t, Event{}, event)
		assert.Equal(t, PhaseAlerting, checkInMonitor.Snapshot().Phase)
	})
}

func TestInactiveLocked(t *testing.T) {
	t.Parallel()

	t.Run("returns true while scheduled", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		checkInMonitor.mu.Lock()
		inactive := checkInMonitor.inactiveLocked()
		checkInMonitor.mu.Unlock()

		assert.True(t, inactive)
	})

	t.Run("returns true without last check-in", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		checkInMonitor.mu.Lock()
		checkInMonitor.phase = PhaseAwaiting
		inactive := checkInMonitor.inactiveLocked()
		checkInMonitor.mu.Unlock()

		assert.True(t, inactive)
	})

	t.Run("returns false with active schedule", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		inactive := checkInMonitor.inactiveLocked()
		checkInMonitor.mu.Unlock()

		assert.False(t, inactive)
	})
}

func TestEnterOverdueLocked(t *testing.T) {
	t.Parallel()

	t.Run("marks monitor overdue", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		now := start.Add(time.Minute)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		schedule := checkInMonitor.scheduleLocked()
		checkInMonitor.enterOverdueLocked(now, schedule)
		phase := checkInMonitor.phase
		overdueSince := checkInMonitor.overdueSince
		checkInMonitor.mu.Unlock()

		assert.Equal(t, PhaseOverdue, phase)
		assert.True(t, overdueSince.Equal(start.Add(time.Minute)))
	})
}

func TestEnterAlertingLocked(t *testing.T) {
	t.Parallel()

	t.Run("marks monitor alerting and returns alerting event", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		now := start.Add(time.Minute + 10*time.Second)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1")
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		schedule := checkInMonitor.scheduleLocked()
		checkInMonitor.overdueSince = schedule.ExpectedBy
		event := checkInMonitor.enterAlertingLocked(now, schedule)
		phase := checkInMonitor.phase
		checkInMonitor.mu.Unlock()

		assert.Equal(t, PhaseAlerting, phase)
		assert.Equal(t, "incident-1", event.IncidentID)
		assert.Equal(t, "notification-alerting-1", event.NotificationID)
		assert.Equal(t, StatusAlerting, event.Status)
		assert.Equal(t, PhaseAlerting, event.Phase)
		assert.False(t, event.Resolved)
	})
}

func TestNewAlertingEventLocked(t *testing.T) {
	t.Parallel()

	t.Run("builds alerting event without changing phase", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		now := start.Add(time.Minute + 10*time.Second)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1", "notification-alerting-1")
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		schedule := checkInMonitor.scheduleLocked()
		checkInMonitor.overdueSince = schedule.ExpectedBy
		event := checkInMonitor.newAlertingEventLocked(now, schedule)
		phase := checkInMonitor.phase
		checkInMonitor.mu.Unlock()

		assert.Equal(t, PhaseAwaiting, phase)
		assert.Equal(t, "incident-1", event.IncidentID)
		assert.Equal(t, "notification-alerting-1", event.NotificationID)
		assert.Equal(t, "default", event.CheckInName)
		assert.True(t, event.LastCheckIn.Equal(start))
		assert.True(t, event.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, event.OverdueSince.Equal(start.Add(time.Minute)))
		assert.True(t, event.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
		assert.True(t, event.Now.Equal(now))
		assert.Equal(t, PhaseAlerting, event.Phase)
		assert.Equal(t, StatusAlerting, event.Status)
		assert.False(t, event.Resolved)
		assert.Empty(t, event.Title)
		assert.Empty(t, event.Text)
	})
}

func TestNewResolvedEventLocked(t *testing.T) {
	t.Parallel()

	t.Run("builds resolved event with existing incident id", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		resolvedAt := start.Add(2 * time.Minute)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("notification-resolved-1")
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		schedule := checkInMonitor.scheduleLocked()
		checkInMonitor.phase = PhaseAlerting
		checkInMonitor.overdueSince = schedule.ExpectedBy
		checkInMonitor.incidentID = "incident-1"
		event := checkInMonitor.newResolvedEventLocked(resolvedAt)
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "incident-1", event.IncidentID)
		assert.Equal(t, "notification-resolved-1", event.NotificationID)
		assert.Equal(t, "default", event.CheckInName)
		assert.True(t, event.LastCheckIn.Equal(start))
		assert.True(t, event.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, event.OverdueSince.Equal(start.Add(time.Minute)))
		assert.True(t, event.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
		assert.True(t, event.Now.Equal(resolvedAt))
		assert.Equal(t, PhaseAwaiting, event.Phase)
		assert.Equal(t, StatusResolved, event.Status)
		assert.True(t, event.Resolved)
		assert.Empty(t, event.Title)
		assert.Empty(t, event.Text)
	})
}

func TestResetIncidentLocked(t *testing.T) {
	t.Parallel()

	t.Run("clears incident and alerting notification ids", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		checkInMonitor.mu.Lock()
		checkInMonitor.incidentID = "incident-1"
		checkInMonitor.alertingNotificationID = "notification-alerting-1"
		checkInMonitor.resetIncidentLocked()
		incidentID := checkInMonitor.incidentID
		notificationID := checkInMonitor.alertingNotificationID
		checkInMonitor.mu.Unlock()

		assert.Empty(t, incidentID)
		assert.Empty(t, notificationID)
	})
}

func TestEnsureIncidentIDLocked(t *testing.T) {
	t.Parallel()

	t.Run("returns existing incident id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		checkInMonitor.mu.Lock()
		checkInMonitor.incidentID = "incident-1"
		id := checkInMonitor.ensureIncidentIDLocked()
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "incident-1", id)
	})

	t.Run("creates missing incident id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1")

		checkInMonitor.mu.Lock()
		id := checkInMonitor.ensureIncidentIDLocked()
		stored := checkInMonitor.incidentID
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "incident-1", id)
		assert.Equal(t, "incident-1", stored)
	})

	t.Run("replaces blank incident id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("incident-1")

		checkInMonitor.mu.Lock()
		checkInMonitor.incidentID = " "
		id := checkInMonitor.ensureIncidentIDLocked()
		stored := checkInMonitor.incidentID
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "incident-1", id)
		assert.Equal(t, "incident-1", stored)
	})
}

func TestEnsureAlertingNotificationIDLocked(t *testing.T) {
	t.Parallel()

	t.Run("returns existing alerting notification id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		checkInMonitor.mu.Lock()
		checkInMonitor.alertingNotificationID = "notification-alerting-1"
		id := checkInMonitor.ensureAlertingNotificationIDLocked()
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "notification-alerting-1", id)
	})

	t.Run("creates missing alerting notification id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("notification-alerting-1")

		checkInMonitor.mu.Lock()
		id := checkInMonitor.ensureAlertingNotificationIDLocked()
		stored := checkInMonitor.alertingNotificationID
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "notification-alerting-1", id)
		assert.Equal(t, "notification-alerting-1", stored)
	})

	t.Run("replaces blank alerting notification id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("notification-alerting-1")

		checkInMonitor.mu.Lock()
		checkInMonitor.alertingNotificationID = " "
		id := checkInMonitor.ensureAlertingNotificationIDLocked()
		stored := checkInMonitor.alertingNotificationID
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "notification-alerting-1", id)
		assert.Equal(t, "notification-alerting-1", stored)
	})
}

func TestNewIDLocked(t *testing.T) {
	t.Parallel()

	t.Run("uses configured generator", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator("id-1")

		checkInMonitor.mu.Lock()
		id := checkInMonitor.newIDLocked()
		checkInMonitor.mu.Unlock()

		assert.Equal(t, "id-1", id)
	})

	t.Run("falls back when generator is nil", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = nil

		checkInMonitor.mu.Lock()
		id := checkInMonitor.newIDLocked()
		checkInMonitor.mu.Unlock()

		assert.NotEmpty(t, id)
	})

	t.Run("falls back when generator returns blank id", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.newID = sequenceIDGenerator(" ")

		checkInMonitor.mu.Lock()
		id := checkInMonitor.newIDLocked()
		checkInMonitor.mu.Unlock()

		assert.NotEmpty(t, id)
		assert.NotEqual(t, " ", id)
	})
}

func TestSnapshotLocked(t *testing.T) {
	t.Parallel()

	t.Run("returns scheduled snapshot while inactive", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		checkInMonitor.mu.Lock()
		snapshot := checkInMonitor.snapshotLocked()
		checkInMonitor.mu.Unlock()

		assert.Equal(t, PhaseScheduled, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.IsZero())
		assert.True(t, snapshot.ExpectedBy.IsZero())
		assert.True(t, snapshot.AlertingAt.IsZero())
	})

	t.Run("returns active snapshot", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		snapshot := checkInMonitor.snapshotLocked()
		checkInMonitor.mu.Unlock()

		assert.Equal(t, PhaseAwaiting, snapshot.Phase)
		assert.True(t, snapshot.LastCheckIn.Equal(start))
		assert.True(t, snapshot.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, snapshot.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
	})
}

func TestScheduleLocked(t *testing.T) {
	t.Parallel()

	t.Run("separates expected and alerting deadlines", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		checkInMonitor.mu.Lock()
		schedule := checkInMonitor.scheduleLocked()
		checkInMonitor.mu.Unlock()

		assert.True(t, schedule.LastCheckIn.Equal(start))
		assert.True(t, schedule.ExpectedBy.Equal(start.Add(time.Minute)))
		assert.True(t, schedule.AlertingAt.Equal(start.Add(time.Minute+10*time.Second)))
	})
}

func TestNextDeadline(t *testing.T) {
	t.Parallel()

	t.Run("returns inactive before first check-in", func(t *testing.T) {
		t.Parallel()

		checkInMonitor := New("default", time.Minute, time.Second, testLogger())

		deadline, active := checkInMonitor.NextDeadline()

		assert.False(t, active)
		assert.True(t, deadline.IsZero())
	})

	t.Run("returns expected deadline while awaiting check-in", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)

		deadline, active := checkInMonitor.NextDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(start.Add(time.Minute)))
	})

	t.Run("returns alerting deadline while overdue", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 10*time.Second, testLogger())
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.Check(start.Add(time.Minute))

		deadline, active := checkInMonitor.NextDeadline()

		assert.True(t, active)
		assert.True(t, deadline.Equal(start.Add(time.Minute+10*time.Second)))
	})

	t.Run("returns inactive after alerting", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 0, testLogger())
		checkInMonitor.RecordCheckIn(start)
		checkInMonitor.Check(start.Add(time.Minute))

		deadline, active := checkInMonitor.NextDeadline()

		assert.False(t, active)
		assert.True(t, deadline.IsZero())
	})

	t.Run("returns inactive for unknown active phase", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		checkInMonitor := New("default", time.Minute, 0, testLogger())

		checkInMonitor.mu.Lock()
		checkInMonitor.phase = Phase("unknown")
		checkInMonitor.lastCheckIn = start
		checkInMonitor.mu.Unlock()

		deadline, active := checkInMonitor.NextDeadline()

		assert.False(t, active)
		assert.True(t, deadline.IsZero())
	})
}

func TestIncidentID(t *testing.T) {
	t.Parallel()

	t.Run("returns explicit incident id", func(t *testing.T) {
		t.Parallel()

		id := incidentID(Event{IncidentID: "incident-1"})

		assert.Equal(t, "incident-1", id)
	})

	t.Run("returns unknown for missing incident id", func(t *testing.T) {
		t.Parallel()

		id := incidentID(Event{})

		assert.Equal(t, "unknown", id)
	})

	t.Run("returns unknown for blank incident id", func(t *testing.T) {
		t.Parallel()

		id := incidentID(Event{IncidentID: " "})

		assert.Equal(t, "unknown", id)
	})
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// sequenceIDGenerator returns IDs in order.
func sequenceIDGenerator(values ...string) idGenerator {
	index := 0
	return func() string {
		if index >= len(values) {
			return values[len(values)-1]
		}

		value := values[index]
		index++
		return value
	}
}
