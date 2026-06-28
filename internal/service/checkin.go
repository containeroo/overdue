package service

import (
	"context"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
)

// CheckInMonitor exposes the check-in monitor behavior used by the check-in service.
type CheckInMonitor interface {
	CheckInName() string
	RecordCheckIn(at time.Time) monitor.RecordResult
	Snapshot() monitor.Snapshot
}

// contextCheckInMonitor is implemented by monitor wrappers that can use a
// request context while recording a check-in, for example to enqueue resolved
// notifications with the same context.
type contextCheckInMonitor interface {
	RecordCheckInContext(ctx context.Context, at time.Time) monitor.RecordResult
}

// CheckIn records incoming check-ins and owns API-side metrics.
type CheckIn struct {
	checkInMonitor CheckInMonitor
	metrics        *metrics.Registry
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
func NewCheckIn(checkInMonitor CheckInMonitor, registry *metrics.Registry) *CheckIn {
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
	record := s.recordCheckIn(ctx, at)
	checkInName := s.checkInMonitor.CheckInName()

	result := RecordCheckInResult{
		CheckInName:   checkInName,
		Snapshot:      record.Snapshot,
		PreviousPhase: record.PreviousPhase,
	}
	s.metrics.IncCheckInReceived(result.CheckInName)

	return result
}

// recordCheckIn records the check-in and uses the request context when the
// configured monitor wrapper supports it.
func (s *CheckIn) recordCheckIn(ctx context.Context, at time.Time) monitor.RecordResult {
	if ctxMonitor, ok := s.checkInMonitor.(contextCheckInMonitor); ok {
		return ctxMonitor.RecordCheckInContext(ctx, at)
	}
	return s.checkInMonitor.RecordCheckIn(at)
}

// Snapshot returns the current check-in monitor snapshot.
func (s *CheckIn) Snapshot() SnapshotResult {
	return SnapshotResult{
		CheckInName: s.checkInMonitor.CheckInName(),
		Snapshot:    s.checkInMonitor.Snapshot(),
	}
}
