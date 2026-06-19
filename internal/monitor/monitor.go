package monitor

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/containeroo/overdue/internal/ids"
	"github.com/containeroo/overdue/internal/utils"
)

// Event describes a check-in lifecycle event used when rendering and sending notifications.
//
// IncidentID identifies one overdue episode and is shared by the alerting and
// resolved notifications. NotificationID identifies one concrete notification
// message and is stable across delivery retries for that message.
//
// The monitor owns factual event data only. Presentation fields such as Title
// and Text are intentionally left empty by the monitor and are filled by the
// notification renderer from the configured title/text templates.
type Event struct {
	IncidentID     string
	NotificationID string
	CheckInName    string
	LastCheckIn    time.Time
	ExpectedBy     time.Time
	OverdueSince   time.Time
	AlertingAt     time.Time
	Now            time.Time
	Phase          Phase
	Status         EventStatus
	Resolved       bool
	Title          string
	Text           string
}

// RecordResult describes the outcome of recording a check-in.
type RecordResult struct {
	Snapshot      Snapshot
	PreviousPhase Phase
	Event         Event
	ShouldNotify  bool
}

// Snapshot describes the current monitor state.
type Snapshot struct {
	LastCheckIn  time.Time
	ExpectedBy   time.Time
	OverdueSince time.Time
	AlertingAt   time.Time
	Phase        Phase
}

// idGenerator creates stable event identity values.
type idGenerator func() string

// Monitor tracks check-in deadlines and notification state.
type Monitor struct {
	mu                     sync.Mutex
	checkInName            string
	expectedEvery          time.Duration
	alertingDelay          time.Duration
	logger                 *slog.Logger
	newID                  idGenerator
	lastCheckIn            time.Time
	overdueSince           time.Time
	incidentID             string
	alertingNotificationID string
	phase                  Phase
}

type (
	// Phase describes the current check-in monitor lifecycle phase.
	Phase string

	// EventStatus describes the status of a notification lifecycle event.
	EventStatus string
)

const (
	// PhaseScheduled means no check-in has been received yet, so the monitor is scheduled but inactive.
	PhaseScheduled Phase = "scheduled"
	// PhaseAwaiting means the monitor is awaiting the next expected check-in.
	PhaseAwaiting Phase = "awaiting"
	// PhaseOverdue means the expected check-in deadline elapsed and the alerting delay is running.
	PhaseOverdue Phase = "overdue"
	// PhaseAlerting means the alerting delay elapsed and notification was sent.
	PhaseAlerting Phase = "alerting"

	// StatusAlerting marks an alerting notification.
	StatusAlerting EventStatus = "alerting"
	// StatusResolved marks a resolved notification.
	StatusResolved EventStatus = "resolved"
)

// New creates a monitor and requires a logger because logging is part of monitor behavior.
func New(
	checkInName string,
	expectedEvery, alertingDelay time.Duration,
	logger *slog.Logger,
) *Monitor {
	if logger == nil {
		panic("monitor logger must not be nil")
	}

	checkInName = utils.DefaultIfZero(strings.TrimSpace(checkInName), "default")

	return &Monitor{
		checkInName:   checkInName,
		expectedEvery: expectedEvery,
		alertingDelay: alertingDelay,
		logger:        logger,
		newID:         ids.MustNewUUIDV7,
		phase:         PhaseScheduled,
	}
}

// CheckInName returns the configured check-in monitor name.
func (m *Monitor) CheckInName() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkInName
}

// RecordCheckIn stores a check-in time, activates or restarts the expected deadline, and returns a resolved event after alerts.
func (m *Monitor) RecordCheckIn(at time.Time) RecordResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	previousPhase := m.phase
	shouldNotify := previousPhase == PhaseAlerting

	var event Event
	if shouldNotify {
		event = m.newResolvedEventLocked(at)
	}

	m.lastCheckIn = at
	m.overdueSince = time.Time{}
	m.resetIncidentLocked()
	m.phase = PhaseAwaiting
	snapshot := m.snapshotLocked()

	return RecordResult{
		Snapshot:      snapshot,
		PreviousPhase: previousPhase,
		Event:         event,
		ShouldNotify:  shouldNotify,
	}
}

// Snapshot returns the current monitor state.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

// CheckResult describes lifecycle events emitted by advancing the monitor clock.
type CheckResult struct {
	Event        Event
	ShouldNotify bool
}

// Check advances monitor state for a given time and returns lifecycle events when needed.
func (m *Monitor) Check(now time.Time) CheckResult {
	event, shouldNotify := m.advance(now)
	return CheckResult{Event: event, ShouldNotify: shouldNotify}
}

// advance moves the monitor through scheduled, awaiting, overdue, and alerting phases.
func (m *Monitor) advance(now time.Time) (event Event, shouldNotify bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inactiveLocked() {
		return Event{}, false
	}

	schedule := m.scheduleLocked()

	switch m.phase {
	case PhaseAwaiting:
		if now.Before(schedule.ExpectedBy) {
			return Event{}, false
		}

		m.enterOverdueLocked(now, schedule)

		if now.Before(schedule.AlertingAt) {
			return Event{}, false
		}

		return m.enterAlertingLocked(now, schedule), true

	case PhaseOverdue:
		if now.Before(schedule.AlertingAt) {
			return Event{}, false
		}

		return m.enterAlertingLocked(now, schedule), true

	default:
		return Event{}, false
	}
}

// inactiveLocked reports whether the monitor has no active check-in schedule while m.mu is already held.
func (m *Monitor) inactiveLocked() bool {
	return m.phase == PhaseScheduled || m.lastCheckIn.IsZero()
}

// enterOverdueLocked marks the monitor as overdue and logs the overdue transition while m.mu is already held.
func (m *Monitor) enterOverdueLocked(now time.Time, schedule schedule) {
	m.phase = PhaseOverdue
	m.overdueSince = schedule.ExpectedBy

	m.logger.Warn(
		"check-in overdue; alerting delay started",
		"expectedEvery", m.expectedEvery.String(),
		"alertingDelay", m.alertingDelay.String(),
		"lastCheckInAge", now.Sub(m.lastCheckIn).Round(time.Millisecond).String(),
		"alertingIn", schedule.AlertingAt.Sub(now).Round(time.Millisecond).String(),
		"lastCheckIn", m.lastCheckIn,
		"expectedBy", schedule.ExpectedBy,
		"alertingAt", schedule.AlertingAt,
	)
}

// enterAlertingLocked marks the monitor as alerting and builds an alerting notification event while m.mu is already held.
func (m *Monitor) enterAlertingLocked(now time.Time, schedule schedule) Event {
	m.phase = PhaseAlerting
	return m.newAlertingEventLocked(now, schedule)
}

// newAlertingEventLocked builds an alerting notification event while m.mu is already held.
func (m *Monitor) newAlertingEventLocked(now time.Time, schedule schedule) Event {
	return Event{
		IncidentID:     m.ensureIncidentIDLocked(),
		NotificationID: m.ensureAlertingNotificationIDLocked(),
		CheckInName:    m.checkInName,
		LastCheckIn:    m.lastCheckIn,
		ExpectedBy:     schedule.ExpectedBy,
		OverdueSince:   m.overdueSince,
		AlertingAt:     schedule.AlertingAt,
		Now:            now,
		Phase:          PhaseAlerting,
		Status:         StatusAlerting,
		Resolved:       false,
	}
}

// newResolvedEventLocked builds a resolved notification event while m.mu is already held.
func (m *Monitor) newResolvedEventLocked(at time.Time) Event {
	schedule := m.scheduleLocked()

	return Event{
		IncidentID:     m.ensureIncidentIDLocked(),
		NotificationID: m.newIDLocked(),
		CheckInName:    m.checkInName,
		LastCheckIn:    m.lastCheckIn,
		ExpectedBy:     schedule.ExpectedBy,
		OverdueSince:   m.overdueSince,
		AlertingAt:     schedule.AlertingAt,
		Now:            at,
		Phase:          PhaseAwaiting,
		Status:         StatusResolved,
		Resolved:       true,
	}
}

// resetIncidentLocked clears incident state while m.mu is already held.
func (m *Monitor) resetIncidentLocked() {
	m.incidentID = ""
	m.alertingNotificationID = ""
}

// ensureIncidentIDLocked returns the current incident ID or creates one while m.mu is already held.
func (m *Monitor) ensureIncidentIDLocked() string {
	if strings.TrimSpace(m.incidentID) == "" {
		m.incidentID = m.newIDLocked()
	}
	return m.incidentID
}

// ensureAlertingNotificationIDLocked returns the current alerting notification ID or creates one while m.mu is already held.
func (m *Monitor) ensureAlertingNotificationIDLocked() string {
	if strings.TrimSpace(m.alertingNotificationID) == "" {
		m.alertingNotificationID = m.newIDLocked()
	}
	return m.alertingNotificationID
}

// newIDLocked returns a non-empty ID while m.mu is already held.
func (m *Monitor) newIDLocked() string {
	generator := m.newID
	if generator == nil {
		generator = ids.MustNewUUIDV7
	}

	id := strings.TrimSpace(generator())
	if id == "" {
		id = ids.MustNewUUIDV7()
	}
	return id
}

// snapshotLocked builds a snapshot while m.mu is already held.
func (m *Monitor) snapshotLocked() Snapshot {
	if m.inactiveLocked() {
		return Snapshot{Phase: PhaseScheduled}
	}

	schedule := m.scheduleLocked()
	return Snapshot{
		LastCheckIn:  schedule.LastCheckIn,
		ExpectedBy:   schedule.ExpectedBy,
		OverdueSince: m.overdueSince,
		AlertingAt:   schedule.AlertingAt,
		Phase:        m.phase,
	}
}

// schedule describes the two check-in deadlines.
//
// ExpectedBy is when the monitor becomes overdue.
// AlertingAt is when the overdue monitor starts alerting and notifications fire.
type schedule struct {
	LastCheckIn time.Time
	ExpectedBy  time.Time
	AlertingAt  time.Time
}

// scheduleLocked returns the current check-in schedule while m.mu is already held.
func (m *Monitor) scheduleLocked() schedule {
	expectedBy := m.lastCheckIn.Add(m.expectedEvery)

	return schedule{
		LastCheckIn: m.lastCheckIn,
		ExpectedBy:  expectedBy,
		AlertingAt:  expectedBy.Add(m.alertingDelay),
	}
}

// NextDeadline returns the next lifecycle deadline for the current phase.
func (m *Monitor) NextDeadline() (deadline time.Time, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inactiveLocked() || m.phase == PhaseAlerting {
		return time.Time{}, false
	}

	schedule := m.scheduleLocked()

	switch m.phase {
	case PhaseAwaiting:
		return schedule.ExpectedBy, true
	case PhaseOverdue:
		return schedule.AlertingAt, true
	default:
		return time.Time{}, false
	}
}

// incidentID returns the event incident ID or a fallback label for defensive logs.
func incidentID(event Event) string {
	if strings.TrimSpace(event.IncidentID) == "" {
		return "unknown"
	}
	return event.IncidentID
}
