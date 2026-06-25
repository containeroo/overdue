package service

import (
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

// CheckIn records incoming check-ins and owns application side effects such as metrics.
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

	service := &CheckIn{
		checkInMonitor: checkInMonitor,
		metrics:        registry,
	}
	registry.SetMonitorSnapshot(checkInMonitor.CheckInName(), checkInMonitor.Snapshot())
	return service
}

// CheckInName returns the configured check-in monitor name.
func (s *CheckIn) CheckInName() string {
	return s.checkInMonitor.CheckInName()
}

// RecordCheckIn records a check-in and returns its new check-in monitor state.
func (s *CheckIn) RecordCheckIn(at time.Time) RecordCheckInResult {
	record := s.checkInMonitor.RecordCheckIn(at)
	checkInName := s.checkInMonitor.CheckInName()

	result := RecordCheckInResult{
		CheckInName:   checkInName,
		Snapshot:      record.Snapshot,
		PreviousPhase: record.PreviousPhase,
	}
	s.metrics.IncCheckInReceived(result.CheckInName)
	s.metrics.SetMonitorSnapshot(result.CheckInName, result.Snapshot)

	return result
}

// Snapshot returns the current check-in monitor snapshot.
func (s *CheckIn) Snapshot() SnapshotResult {
	result := SnapshotResult{
		CheckInName: s.checkInMonitor.CheckInName(),
		Snapshot:    s.checkInMonitor.Snapshot(),
	}
	s.metrics.SetMonitorSnapshot(result.CheckInName, result.Snapshot)

	return result
}
