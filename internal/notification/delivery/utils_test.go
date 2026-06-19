package delivery

import (
	"testing"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationKey(t *testing.T) {
	t.Parallel()
	t.Run("returns trimmed notification id", func(t *testing.T) {
		t.Parallel()

		key, err := NotificationKey(monitor.Event{NotificationID: " notification-1 "})

		require.NoError(t, err)
		assert.Equal(t, "notification-1", key)
	})

	t.Run("rejects empty notification id", func(t *testing.T) {
		t.Parallel()

		_, err := NotificationKey(monitor.Event{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing notification id")
	})

	t.Run("rejects blank notification id", func(t *testing.T) {
		t.Parallel()

		_, err := NotificationKey(monitor.Event{NotificationID: " \t\n "})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing notification id")
	})
}
