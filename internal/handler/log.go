package handler

import (
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/service"
)

// logCheckInReceived writes the lifecycle log for an accepted check-in request.
func (a *API) logCheckInReceived(result service.RecordCheckInResult, at time.Time, request RequestMetadata) {
	fields := checkInLogFields(result.Snapshot, at, request)

	switch result.PreviousPhase {
	case monitor.PhaseScheduled:
		a.logger.Info("first check-in received; check-in monitor active", fields...)
	case monitor.PhaseOverdue:
		a.logger.Info(
			"check-in received while overdue; next deadline scheduled",
			append(fields, "previousPhase", result.PreviousPhase)...,
		)
	case monitor.PhaseAlerting:
		a.logger.Info(
			"check-in received while alerting; next deadline scheduled",
			append(fields, "previousPhase", result.PreviousPhase)...,
		)
	default:
		a.logger.Info("check-in received; next deadline scheduled", fields...)
	}
}

// logSnapshotRequested writes the lifecycle log for a status request.
func (a *API) logSnapshotRequested(result service.SnapshotResult, request RequestMetadata) {
	fields := request.LogFields()
	fields = append(
		fields,
		"phase", result.Snapshot.Phase,
		"lastCheckIn", result.Snapshot.LastCheckIn,
		"expectedBy", result.Snapshot.ExpectedBy,
	)

	a.logger.Debug("snapshot requested", fields...)
}

// checkInLogFields returns structured fields for accepted check-in lifecycle logs.
func checkInLogFields(snapshot monitor.Snapshot, at time.Time, request RequestMetadata) []any {
	fields := request.LogFields()
	fields = append(
		fields,
		"phase", snapshot.Phase,
		"lastCheckIn", at,
		"expectedBy", snapshot.ExpectedBy,
		"alertingAt", snapshot.AlertingAt,
	)

	if !snapshot.LastCheckIn.IsZero() && !snapshot.ExpectedBy.IsZero() {
		fields = append(fields, "expectedEvery", snapshot.ExpectedBy.Sub(snapshot.LastCheckIn).String())
	}
	if !snapshot.ExpectedBy.IsZero() && !snapshot.AlertingAt.IsZero() {
		fields = append(fields, "alertingDelay", snapshot.AlertingAt.Sub(snapshot.ExpectedBy).String())
	}

	return fields
}
