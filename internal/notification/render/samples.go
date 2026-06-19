package render

import (
	"time"

	"github.com/containeroo/overdue/internal/monitor"
)

// SampleAlertingEvent returns a complete alerting event for startup template validation.
func SampleAlertingEvent() monitor.Event {
	lastCheckIn := time.Date(2026, 6, 19, 14, 28, 22, 0, time.FixedZone("CEST", 2*60*60))
	expectedBy := lastCheckIn.Add(5 * time.Second)
	alertingAt := expectedBy.Add(3 * time.Second)
	return monitor.Event{
		IncidentID:     "sample-incident",
		NotificationID: "sample-alerting-notification",
		CheckInName:    "default",
		LastCheckIn:    lastCheckIn,
		ExpectedBy:     expectedBy,
		OverdueSince:   expectedBy,
		AlertingAt:     alertingAt,
		Now:            alertingAt,
		Phase:          monitor.PhaseAlerting,
		Status:         monitor.StatusAlerting,
		Resolved:       false,
	}
}

// SampleResolvedEvent returns a complete resolved event for startup template validation.
func SampleResolvedEvent() monitor.Event {
	event := SampleAlertingEvent()
	event.NotificationID = "sample-resolved-notification"
	event.Now = event.AlertingAt.Add(5 * time.Second)
	event.Phase = monitor.PhaseAwaiting
	event.Status = monitor.StatusResolved
	event.Resolved = true
	return event
}
