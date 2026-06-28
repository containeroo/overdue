package metrics

import (
	"net/http"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	monitorPhaseInactive float64 = 0
	monitorPhaseActive   float64 = 1
)

var monitorPhases = []monitor.Phase{
	monitor.PhaseScheduled,
	monitor.PhaseAwaiting,
	monitor.PhaseOverdue,
	monitor.PhaseAlerting,
}

// Registry holds Prometheus metrics for the app.
type Registry struct {
	registry                    *prometheus.Registry
	monitorPhase                *prometheus.GaugeVec
	checkInsReceivedTotal       *prometheus.CounterVec
	monitorLastCheckInTimestamp *prometheus.GaugeVec
	monitorExpectedByTimestamp  *prometheus.GaugeVec
	monitorAlertingAtTimestamp  *prometheus.GaugeVec
	notificationsQueuedTotal    *prometheus.CounterVec
	notificationsSkippedTotal   *prometheus.CounterVec
	notificationsQueueFailed    *prometheus.CounterVec
}

// NewRegistry builds a new Prometheus metrics registry.
func NewRegistry() *Registry {
	monitorPhase := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overdue_monitor_phase",
			Help: "Current monitor phase. The active phase has value 1, inactive phases have value 0.",
		},
		[]string{"check_in", "phase"},
	)
	checkInsReceivedTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "overdue_checkins_received_total",
			Help: "Total number of received check-ins per monitor.",
		},
		[]string{"check_in"},
	)
	monitorLastCheckInTimestamp := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overdue_monitor_last_checkin_timestamp_seconds",
			Help: "Unix timestamp of the last received check-in per monitor. The value is 0 when no check-in has been received.",
		},
		[]string{"check_in"},
	)
	monitorExpectedByTimestamp := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overdue_monitor_expected_by_timestamp_seconds",
			Help: "Unix timestamp when the next check-in is expected per monitor. The value is 0 when no deadline is active.",
		},
		[]string{"check_in"},
	)
	monitorAlertingAtTimestamp := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overdue_monitor_alerting_at_timestamp_seconds",
			Help: "Unix timestamp when the monitor starts alerting per monitor. The value is 0 when no alerting deadline is active.",
		},
		[]string{"check_in"},
	)
	notificationsQueuedTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "overdue_notifications_queued_total",
			Help: "Total number of notifications accepted by the queue per monitor and status.",
		},
		[]string{"check_in", "status"},
	)
	notificationsSkippedTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "overdue_notifications_skipped_total",
			Help: "Total number of notifications skipped before queueing per monitor, status, and reason.",
		},
		[]string{"check_in", "status", "reason"},
	)
	notificationsQueueFailed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "overdue_notifications_queue_failed_total",
			Help: "Total number of notifications that failed to enqueue per monitor and status.",
		},
		[]string{"check_in", "status"},
	)

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		monitorPhase,
		checkInsReceivedTotal,
		monitorLastCheckInTimestamp,
		monitorExpectedByTimestamp,
		monitorAlertingAtTimestamp,
		notificationsQueuedTotal,
		notificationsSkippedTotal,
		notificationsQueueFailed,
	)

	return &Registry{
		registry:                    reg,
		monitorPhase:                monitorPhase,
		checkInsReceivedTotal:       checkInsReceivedTotal,
		monitorLastCheckInTimestamp: monitorLastCheckInTimestamp,
		monitorExpectedByTimestamp:  monitorExpectedByTimestamp,
		monitorAlertingAtTimestamp:  monitorAlertingAtTimestamp,
		notificationsQueuedTotal:    notificationsQueuedTotal,
		notificationsSkippedTotal:   notificationsSkippedTotal,
		notificationsQueueFailed:    notificationsQueueFailed,
	}
}

// SetMonitorSnapshot updates all monitor gauges from the current monitor snapshot.
func (r *Registry) SetMonitorSnapshot(checkIn string, snapshot monitor.Snapshot) {
	r.setActiveMonitorPhase(checkIn, snapshot.Phase)
	r.monitorLastCheckInTimestamp.WithLabelValues(checkIn).Set(timestampValue(snapshot.LastCheckIn))
	r.monitorExpectedByTimestamp.WithLabelValues(checkIn).Set(timestampValue(snapshot.ExpectedBy))
	r.monitorAlertingAtTimestamp.WithLabelValues(checkIn).Set(timestampValue(snapshot.AlertingAt))
}

// IncCheckInReceived increments the received check-in counter for a monitor.
func (r *Registry) IncCheckInReceived(checkIn string) {
	r.checkInsReceivedTotal.WithLabelValues(checkIn).Inc()
}

// IncNotificationQueued increments the queued notification counter.
func (r *Registry) IncNotificationQueued(checkIn string, status monitor.EventStatus) {
	r.notificationsQueuedTotal.WithLabelValues(checkIn, string(status)).Inc()
}

// IncNotificationSkipped increments the skipped notification counter.
func (r *Registry) IncNotificationSkipped(checkIn string, status monitor.EventStatus, reason string) {
	r.notificationsSkippedTotal.WithLabelValues(checkIn, string(status), reason).Inc()
}

// IncNotificationQueueFailed increments the queue failure notification counter.
func (r *Registry) IncNotificationQueueFailed(checkIn string, status monitor.EventStatus) {
	r.notificationsQueueFailed.WithLabelValues(checkIn, string(status)).Inc()
}

// Metrics returns the Prometheus metrics handler.
func (r *Registry) Metrics() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}

// setActiveMonitorPhase updates the one-hot phase gauge for a monitor.
// The active phase is set to 1 and all other known phases are reset to 0.
func (r *Registry) setActiveMonitorPhase(checkIn string, phase monitor.Phase) {
	for _, known := range monitorPhases {
		value := monitorPhaseInactive
		if phase == known {
			value = monitorPhaseActive
		}
		r.monitorPhase.WithLabelValues(checkIn, string(known)).Set(value)
	}
}

// timestampValue returns a Unix timestamp in seconds or 0 for zero timestamps.
func timestampValue(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.Unix())
}
