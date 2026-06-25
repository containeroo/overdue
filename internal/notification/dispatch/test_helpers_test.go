package dispatch

import (
	"context"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
)

type recordingTarget struct {
	called int
	err    error
	target target.Target
}

// Notify records calls for assertions.
func (n *recordingTarget) Notify(_ context.Context, _ monitor.Event) error {
	n.called++
	return n.err
}

// Target returns test target metadata.
func (n *recordingTarget) Target() target.Target {
	return n.target
}

// testEvent returns a alerting event fixture.
func testEvent() monitor.Event {
	return monitor.Event{
		IncidentID:     "incident",
		NotificationID: "notification-alerting",
		CheckInName:    "default",
		LastCheckIn:    time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC),
		ExpectedBy:     time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC),
		OverdueSince:   time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC),
		AlertingAt:     time.Date(2026, 6, 19, 9, 5, 0, 0, time.UTC),
		Now:            time.Date(2026, 6, 19, 9, 5, 1, 0, time.UTC),
		Phase:          monitor.PhaseAlerting,
		Status:         monitor.StatusAlerting,
		Resolved:       false,
	}
}
