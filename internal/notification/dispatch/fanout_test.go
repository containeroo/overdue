package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/delivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFanout(t *testing.T) {
	t.Parallel()
	t.Run("continues fan out after errors and skips delivered on retry", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		first := &recordingNotifier{err: wantErr}
		second := &recordingNotifier{}
		fanout := New([]delivery.Notifier{first, second})

		err := fanout.Notify(context.Background(), testEvent())
		require.Error(t, err)
		assert.ErrorIs(t, err, wantErr)
		assert.Equal(t, 1, first.called)
		assert.Equal(t, 1, second.called)

		err = fanout.Notify(context.Background(), testEvent())
		require.Error(t, err)
		assert.Equal(t, 2, first.called)
		assert.Equal(t, 1, second.called)
	})

	t.Run("reports target status without errors", func(t *testing.T) {
		t.Parallel()

		first := &recordingNotifier{
			err:    errors.New("secret smtp failure"),
			target: delivery.Target{Type: "email", Name: "email"},
		}
		second := &recordingNotifier{
			target: delivery.Target{Type: "webhook", Name: "teams"},
		}
		fanout := New([]delivery.Notifier{first, second})

		err := fanout.Notify(context.Background(), testEvent())
		require.Error(t, err)

		status := fanout.NotificationStatus()
		assert.Equal(t, delivery.StatusPartialFailure, status.Status)
		assert.Equal(t, 2, status.Total)
		assert.Equal(t, 1, status.Delivered)
		assert.Equal(t, 1, status.Failed)
		assert.Equal(t, 1, status.Pending)
		require.Len(t, status.Targets, 2)

		assert.Equal(t, "email", status.Targets[0].Type)
		assert.Equal(t, "email", status.Targets[0].Name)
		assert.Equal(t, delivery.StatusFailed, status.Targets[0].Status)
		assert.NotNil(t, status.Targets[0].LastAttemptAt)
		assert.Nil(t, status.Targets[0].LastDeliveredAt)

		assert.Equal(t, "webhook", status.Targets[1].Type)
		assert.Equal(t, "teams", status.Targets[1].Name)
		assert.Equal(t, delivery.StatusDelivered, status.Targets[1].Status)
		assert.NotNil(t, status.Targets[1].LastAttemptAt)
		assert.NotNil(t, status.Targets[1].LastDeliveredAt)
	})

	t.Run("reports delivered after retry succeeds", func(t *testing.T) {
		t.Parallel()

		first := &recordingNotifier{
			err:    errors.New("temporary failure"),
			target: delivery.Target{Type: "email", Name: "email"},
		}
		second := &recordingNotifier{
			target: delivery.Target{Type: "webhook", Name: "teams"},
		}
		fanout := New([]delivery.Notifier{first, second})

		require.Error(t, fanout.Notify(context.Background(), testEvent()))
		first.err = nil
		require.NoError(t, fanout.Notify(context.Background(), testEvent()))

		status := fanout.NotificationStatus()
		assert.Equal(t, delivery.StatusDelivered, status.Status)
		assert.Equal(t, 2, status.Delivered)
		assert.Equal(t, 0, status.Failed)
		assert.Equal(t, 0, status.Pending)
		require.Len(t, status.Targets, 2)
		assert.Equal(t, delivery.StatusDelivered, status.Targets[0].Status)
		assert.Equal(t, delivery.StatusDelivered, status.Targets[1].Status)
	})

	t.Run("reports skipped target as skipped", func(t *testing.T) {
		t.Parallel()

		notifier := &recordingNotifier{
			err:    delivery.ErrSkipped,
			target: delivery.Target{Type: "webhook", Name: "teams"},
		}
		fanout := New([]delivery.Notifier{notifier})

		require.NoError(t, fanout.Notify(context.Background(), testEvent()))

		status := fanout.NotificationStatus()
		assert.Equal(t, delivery.StatusDelivered, status.Status)
		assert.Equal(t, 0, status.Delivered)
		assert.Equal(t, 1, status.Skipped)
		require.Len(t, status.Targets, 1)
		assert.Equal(t, delivery.StatusSkipped, status.Targets[0].Status)
		assert.NotNil(t, status.Targets[0].LastAttemptAt)
		assert.Nil(t, status.Targets[0].LastDeliveredAt)
	})

	t.Run("rejects event without notification id", func(t *testing.T) {
		t.Parallel()

		fanout := New([]delivery.Notifier{&recordingNotifier{}})
		event := testEvent()
		event.NotificationID = ""

		err := fanout.Notify(context.Background(), event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing notification id")
	})
}

func TestRetryError_Error(t *testing.T) {
	t.Parallel()
	t.Run("returns wrapped error string", func(t *testing.T) {
		t.Parallel()

		err := &RetryError{Err: errors.New("boom")}

		assert.Equal(t, "boom", err.Error())
	})
}

func TestRetryError_Unwrap(t *testing.T) {
	t.Parallel()
	t.Run("returns wrapped error", func(t *testing.T) {
		t.Parallel()

		want := errors.New("boom")
		err := &RetryError{Err: want}

		assert.ErrorIs(t, err.Unwrap(), want)
	})
}

func TestRetryError_RetryAfter(t *testing.T) {
	t.Parallel()
	t.Run("returns retry wait", func(t *testing.T) {
		t.Parallel()

		err := &RetryError{RetryWait: 5 * time.Second}

		assert.Equal(t, 5*time.Second, err.RetryAfter())
	})
}

func TestRetryError_NotificationStats(t *testing.T) {
	t.Parallel()
	t.Run("returns notification stats", func(t *testing.T) {
		t.Parallel()

		err := &RetryError{Delivered: 1, Failed: 2, Pending: 3}

		delivered, failed, pending := err.NotificationStats()

		assert.Equal(t, 1, delivered)
		assert.Equal(t, 2, failed)
		assert.Equal(t, 3, pending)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("copies notifiers and initializes defaults", func(t *testing.T) {
		t.Parallel()

		notifiers := []delivery.Notifier{&recordingNotifier{}}

		fanout := New(notifiers)
		notifiers[0] = nil

		require.Len(t, fanout.Notifiers, 1)
		require.NotNil(t, fanout.Notifiers[0])
		assert.Equal(t, defaultInitialNotificationBackoff, fanout.InitialBackoff)
		assert.Equal(t, defaultMaxNotificationBackoff, fanout.MaxBackoff)
	})
}

func TestFanout_nextBackoff(t *testing.T) {
	t.Parallel()
	t.Run("uses exponential backoff", func(t *testing.T) {
		t.Parallel()

		fanout := New(nil)
		fanout.InitialBackoff = time.Second
		fanout.MaxBackoff = 10 * time.Second
		key := deliveryKey(testEvent().NotificationID)

		assert.Equal(t, time.Second, fanout.nextBackoff(key))
		assert.Equal(t, 2*time.Second, fanout.nextBackoff(key))
		assert.Equal(t, 4*time.Second, fanout.nextBackoff(key))
	})

	t.Run("caps at max backoff", func(t *testing.T) {
		t.Parallel()

		fanout := New(nil)
		fanout.InitialBackoff = 5 * time.Second
		fanout.MaxBackoff = 6 * time.Second
		key := deliveryKey(testEvent().NotificationID)

		assert.Equal(t, 5*time.Second, fanout.nextBackoff(key))
		assert.Equal(t, 6*time.Second, fanout.nextBackoff(key))
	})
}

func TestFanout_initialBackoffLocked(t *testing.T) {
	t.Parallel()
	t.Run("uses configured value", func(t *testing.T) {
		t.Parallel()

		fanout := &Fanout{InitialBackoff: 5 * time.Second}

		assert.Equal(t, 5*time.Second, fanout.initialBackoffLocked())
	})

	t.Run("uses default value", func(t *testing.T) {
		t.Parallel()

		fanout := &Fanout{}

		assert.Equal(t, defaultInitialNotificationBackoff, fanout.initialBackoffLocked())
	})
}

func TestFanout_maxBackoffLocked(t *testing.T) {
	t.Parallel()
	t.Run("uses configured value", func(t *testing.T) {
		t.Parallel()

		fanout := &Fanout{MaxBackoff: 5 * time.Second}

		assert.Equal(t, 5*time.Second, fanout.maxBackoffLocked())
	})

	t.Run("uses default value", func(t *testing.T) {
		t.Parallel()

		fanout := &Fanout{}

		assert.Equal(t, defaultMaxNotificationBackoff, fanout.maxBackoffLocked())
	})
}

func TestFanout_eventStateLocked(t *testing.T) {
	t.Parallel()
	t.Run("tracks delivery and attempts for one event", func(t *testing.T) {
		t.Parallel()

		fanout := New([]delivery.Notifier{&recordingNotifier{}})
		key := deliveryKey(testEvent().NotificationID)

		fanout.mu.Lock()
		state := fanout.eventStateLocked(key)
		state.delivered[0] = true
		state.attempts = 2
		fanout.Notifiers = append(fanout.Notifiers, &recordingNotifier{})
		state = fanout.eventStateLocked(key)
		fanout.mu.Unlock()

		require.Len(t, state.delivered, 2)
		assert.True(t, state.delivered[0])
		assert.False(t, state.delivered[1])
		assert.Equal(t, 2, state.attempts)
	})
}
