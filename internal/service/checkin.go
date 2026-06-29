package service

import (
	"context"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
)

// CheckInMonitor exposes the check-in monitor behavior used by the check-in service.
type CheckInMonitor interface {
	CheckInName() string
	RecordCheckInContext(ctx context.Context, at time.Time) monitor.RecordResult
	Snapshot() monitor.Snapshot
}

// MetricsRecorder records check-in service metrics.
type MetricsRecorder interface {
	IncCheckInReceived(checkIn string)
}

// CheckIn records incoming check-ins and owns API-side metrics.
type CheckIn struct {
	checkInMonitor CheckInMonitor
	metrics        MetricsRecorder
}

// RecordCheckInResult describes the outcome of recording a check-in.
type RecordCheckInResult struct {
	CheckInName   string
	Snapshot      monitor.Snapshot
	PreviousPhase monitor.Phase
}

// SnapshotResult describes a check-in monitor snapshot.
type SnapshotResult struct {
	CheckInName string
	Snapshot    monitor.Snapshot
}

// NewCheckIn creates a check-in service.
func NewCheckIn(checkInMonitor CheckInMonitor, registry MetricsRecorder) *CheckIn {
	if checkInMonitor == nil {
		panic("check-in monitor must not be nil")
	}
	if registry == nil {
		panic("check-in metrics registry must not be nil")
	}

	return &CheckIn{
		checkInMonitor: checkInMonitor,
		metrics:        registry,
	}
}

// CheckInName returns the configured check-in monitor name.
func (s *CheckIn) CheckInName() string {
	return s.checkInMonitor.CheckInName()
}

// RecordCheckIn records a check-in and returns its new check-in monitor state.
func (s *CheckIn) RecordCheckIn(ctx context.Context, at time.Time) RecordCheckInResult {
	record := s.checkInMonitor.RecordCheckInContext(ctx, at)
	checkInName := s.checkInMonitor.CheckInName()

	result := RecordCheckInResult{
		CheckInName:   checkInName,
		Snapshot:      record.Snapshot,
		PreviousPhase: record.PreviousPhase,
	}
	s.metrics.IncCheckInReceived(result.CheckInName)

	return result
}

// Snapshot returns the current check-in monitor snapshot.
func (s *CheckIn) Snapshot() SnapshotResult {
	return SnapshotResult{
		CheckInName: s.checkInMonitor.CheckInName(),
		Snapshot:    s.checkInMonitor.Snapshot(),
	}
}
