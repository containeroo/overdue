package dispatch

import (
	"context"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
)

// None is a dispatcher that intentionally sends nothing.
type None struct{}

// Notify implements target.Dispatcher without sending anything.
func (None) Notify(context.Context, monitor.Event) error {
	return nil
}

// NotificationStatus reports an idle notification state.
func (None) NotificationStatus() target.Status {
	return target.Status{Status: target.StatusIdle}
}
