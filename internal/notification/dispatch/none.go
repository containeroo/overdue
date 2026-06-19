package dispatch

import (
	"context"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/delivery"
)

// None is a notifier that intentionally sends nothing.
type None struct{}

var (
	_ delivery.Notifier       = None{}
	_ delivery.StatusProvider = None{}
)

// Notify implements delivery.Notifier without sending a delivery.
func (None) Notify(context.Context, monitor.Event) error {
	return nil
}

// NotificationStatus reports an idle notification state.
func (None) NotificationStatus() delivery.Status {
	return delivery.Status{Status: delivery.StatusIdle}
}
