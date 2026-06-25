package notify

import (
	"testing"

	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
)

// TestNewEvent tests Event construction.
func TestNewEvent(t *testing.T) {
	t.Parallel()

	t.Run("copies receiver ids", func(t *testing.T) {
		t.Parallel()

		receivers := []kit.ReceiverID{"ops"}
		event := NewEvent(monitor.Event{NotificationID: "n1"}, receivers)
		receivers[0] = "changed"

		assert.Equal(t, "n1", event.ID())
		assert.Equal(t, []kit.ReceiverID{"ops"}, event.ReceiverIDs())
	})
}

// TestEvent_ID tests ID retrieval.
func TestEvent_ID(t *testing.T) {
	t.Parallel()

	t.Run("returns empty for nil event", func(t *testing.T) {
		t.Parallel()

		var event *Event

		assert.Empty(t, event.ID())
	})
}

// TestEvent_ReceiverIDs tests explicit receiver routing.
func TestEvent_ReceiverIDs(t *testing.T) {
	t.Parallel()

	t.Run("returns nil without explicit receivers", func(t *testing.T) {
		t.Parallel()

		event := &Event{}

		assert.Nil(t, event.ReceiverIDs())
	})

	t.Run("returns a defensive copy", func(t *testing.T) {
		t.Parallel()

		event := &Event{Receivers: []kit.ReceiverID{"ops"}}
		ids := event.ReceiverIDs()
		ids[0] = "changed"

		assert.Equal(t, []kit.ReceiverID{"ops"}, event.ReceiverIDs())
	})
}

// TestEvent_Data tests template data conversion.
func TestEvent_Data(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for nil event", func(t *testing.T) {
		t.Parallel()

		var event *Event

		assert.Nil(t, event.Data("ops", nil, "subject"))
	})
}
