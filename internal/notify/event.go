package notify

import (
	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/overdue/internal/monitor"
)

// Event wraps a monitor event as a notifykit notification.
type Event struct {
	MonitorEvent monitor.Event
	Receivers    []kit.ReceiverID
}

// NewEvent constructs an Event notification.
func NewEvent(monitorEvent monitor.Event, receivers []kit.ReceiverID) *Event {
	return &Event{
		MonitorEvent: monitorEvent,
		Receivers:    append([]kit.ReceiverID(nil), receivers...),
	}
}

// ID returns the stable notification id.
func (e *Event) ID() string {
	if e == nil {
		return ""
	}
	return e.MonitorEvent.NotificationID
}

// ReceiverIDs returns explicit receiver routing.
func (e *Event) ReceiverIDs() []kit.ReceiverID {
	if e == nil || len(e.Receivers) == 0 {
		return nil
	}
	return append([]kit.ReceiverID(nil), e.Receivers...)
}

// Data returns the receiver-scoped template data.
func (e *Event) Data(receiver string, vars map[string]any, subject string) any {
	if e == nil {
		return nil
	}
	return NewData(e.MonitorEvent, receiver, vars, subject)
}
