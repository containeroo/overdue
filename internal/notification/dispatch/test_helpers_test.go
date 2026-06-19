package dispatch

import (
	"context"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/delivery"
)

type recordingNotifier struct {
	called int
	err    error
	target delivery.Target
}

// Notify records calls for assertions.
func (n *recordingNotifier) Notify(_ context.Context, _ monitor.Event) error {
	n.called++
	return n.err
}

// NotificationTarget returns test target metadata.
func (n *recordingNotifier) NotificationTarget() delivery.Target {
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
