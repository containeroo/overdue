package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
)

type (
	// DeliveryStatus describes aggregate or per-target notification delivery state.
	DeliveryStatus string
)

const (
	// StatusIdle means no notification delivery has been attempted yet.
	StatusIdle DeliveryStatus = "idle"
	// StatusPending means a notification delivery attempt is in progress or waiting to retry.
	StatusPending DeliveryStatus = "pending"
	// StatusDelivered means notification delivery completed successfully.
	StatusDelivered DeliveryStatus = "delivered"
	// StatusFailed means notification delivery failed and is still pending.
	StatusFailed DeliveryStatus = "failed"
	// StatusSkipped means notification delivery was intentionally skipped.
	StatusSkipped DeliveryStatus = "skipped"
	// StatusPartialFailure means at least one target delivered while another target is still pending.
	StatusPartialFailure DeliveryStatus = "partial_failure"
)

// ErrSkipped marks an intentionally skipped notification delivery.
var ErrSkipped = errors.New("notification skipped")

// Notifier sends check-in lifecycle events.
type Notifier interface {
	Notify(ctx context.Context, event monitor.Event) error
}

// Target identifies one configured notification target.
type Target struct {
	Type string
	Name string
}

// Targeter is implemented by notifiers that expose public target metadata.
type Targeter interface {
	NotificationTarget() Target
}

// StatusProvider is implemented by notifiers that expose delivery status.
type StatusProvider interface {
	NotificationStatus() Status
}

// TargetStatus describes delivery state for one configured target.
type TargetStatus struct {
	Type            string
	Name            string
	Status          DeliveryStatus
	LastAttemptAt   *time.Time
	LastDeliveredAt *time.Time
}

// Status describes aggregate notification delivery state.
type Status struct {
	Status    DeliveryStatus
	Total     int
	Delivered int
	Failed    int
	Pending   int
	Skipped   int
	Targets   []TargetStatus
}
