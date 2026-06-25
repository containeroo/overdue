package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFanoutNotify(t *testing.T) {
	t.Parallel()

	t.Run("continues fan out after errors and skips delivered targets on retry", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		first := &recordingTarget{err: wantErr, target: target.Target{Type: "email", Name: "primary"}}
		second := &recordingTarget{target: target.Target{Type: "webhook", Name: "ops"}}
		fanout := New([]target.Notifier{first, second})

		err := fanout.Notify(context.Background(), testEvent())
		require.Error(t, err)
		assert.ErrorIs(t, err, wantErr)
		assert.Contains(t, err.Error(), `email "primary"`)
		assert.Equal(t, 1, first.called)
		assert.Equal(t, 1, second.called)

		err = fanout.Notify(context.Background(), testEvent())
		require.Error(t, err)
		assert.Equal(t, 2, first.called)
		assert.Equal(t, 1, second.called)
	})

	t.Run("reports delivered after retry succeeds", func(t *testing.T) {
		t.Parallel()

		first := &recordingTarget{err: errors.New("temporary failure"), target: target.Target{Type: "email", Name: "primary"}}
		second := &recordingTarget{target: target.Target{Type: "webhook", Name: "ops"}}
		fanout := New([]target.Notifier{first, second})

		require.Error(t, fanout.Notify(context.Background(), testEvent()))
		first.err = nil
		require.NoError(t, fanout.Notify(context.Background(), testEvent()))

		status := fanout.NotificationStatus()
		assert.Equal(t, target.StatusDelivered, status.Status)
		assert.Equal(t, 2, status.Delivered)
		assert.Equal(t, 0, status.Failed)
		assert.Equal(t, 0, status.Pending)
		require.Len(t, status.Targets, 2)
		assert.Equal(t, target.StatusDelivered, status.Targets[0].Status)
		assert.Equal(t, target.StatusDelivered, status.Targets[1].Status)
	})

	t.Run("records skipped targets as successful for the current event", func(t *testing.T) {
		t.Parallel()

		skipped := &recordingTarget{err: target.ErrSkipped, target: target.Target{Type: "webhook", Name: "ops"}}
		fanout := New([]target.Notifier{skipped})

		require.NoError(t, fanout.Notify(context.Background(), testEvent()))

		status := fanout.NotificationStatus()
		assert.Equal(t, target.StatusDelivered, status.Status)
		assert.Equal(t, 0, status.Delivered)
		assert.Equal(t, 1, status.Skipped)
		require.Len(t, status.Targets, 1)
		assert.Equal(t, target.StatusSkipped, status.Targets[0].Status)
		assert.NotNil(t, status.Targets[0].LastAttemptAt)
		assert.Nil(t, status.Targets[0].LastDeliveredAt)
	})

	t.Run("rejects event without notification id", func(t *testing.T) {
		t.Parallel()

		fanout := New([]target.Notifier{&recordingTarget{target: target.Target{Type: "webhook", Name: "ops"}}})
		event := testEvent()
		event.NotificationID = ""

		err := fanout.Notify(context.Background(), event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing notification id")
	})
}

func TestFanoutNotificationStatus(t *testing.T) {
	t.Parallel()

	t.Run("reports target status without errors", func(t *testing.T) {
		t.Parallel()

		first := &recordingTarget{
			err:    errors.New("secret smtp failure"),
			target: target.Target{Type: "email", Name: "primary"},
		}
		second := &recordingTarget{
			target: target.Target{Type: "webhook", Name: "ops"},
		}
		fanout := New([]target.Notifier{first, second})

		err := fanout.Notify(context.Background(), testEvent())
		require.Error(t, err)

		status := fanout.NotificationStatus()
		assert.Equal(t, target.StatusPartialFailure, status.Status)
		assert.Equal(t, 2, status.Total)
		assert.Equal(t, 1, status.Delivered)
		assert.Equal(t, 1, status.Failed)
		assert.Equal(t, 1, status.Pending)
		require.Len(t, status.Targets, 2)

		assert.Equal(t, "email", status.Targets[0].Type)
		assert.Equal(t, "primary", status.Targets[0].Name)
		assert.Equal(t, target.StatusFailed, status.Targets[0].Status)
		assert.NotNil(t, status.Targets[0].LastAttemptAt)
		assert.Nil(t, status.Targets[0].LastDeliveredAt)

		assert.Equal(t, "webhook", status.Targets[1].Type)
		assert.Equal(t, "ops", status.Targets[1].Name)
		assert.Equal(t, target.StatusDelivered, status.Targets[1].Status)
		assert.NotNil(t, status.Targets[1].LastAttemptAt)
		assert.NotNil(t, status.Targets[1].LastDeliveredAt)
	})

	t.Run("normalizes empty target metadata", func(t *testing.T) {
		t.Parallel()

		fanout := New([]target.Notifier{&recordingTarget{}})

		status := fanout.NotificationStatus()

		require.Len(t, status.Targets, 1)
		assert.Equal(t, "unknown", status.Targets[0].Type)
		assert.Equal(t, "unknown-0", status.Targets[0].Name)
	})

	t.Run("nil fanout is idle", func(t *testing.T) {
		t.Parallel()

		var fanout *Fanout

		assert.Equal(t, target.Status{Status: target.StatusIdle}, fanout.NotificationStatus())
	})
}

func TestFanoutTargets(t *testing.T) {
	t.Parallel()

	t.Run("returns configured targets in delivery order", func(t *testing.T) {
		t.Parallel()

		configured := []target.Notifier{
			&recordingTarget{target: target.Target{Type: "webhook", Name: "ops"}},
			&recordingTarget{target: target.Target{Type: "email", Name: "primary"}},
		}
		fanout := New(configured)
		configured[0] = &recordingTarget{target: target.Target{Type: "changed", Name: "changed"}}

		assert.Equal(t, []target.Target{
			{Type: "webhook", Name: "ops"},
			{Type: "email", Name: "primary"},
		}, fanout.Targets())
	})

	t.Run("nil fanout returns nil", func(t *testing.T) {
		t.Parallel()

		var fanout *Fanout

		assert.Nil(t, fanout.Targets())
	})
}

func TestRetryError(t *testing.T) {
	t.Parallel()

	t.Run("exposes retry details", func(t *testing.T) {
		t.Parallel()

		want := errors.New("boom")
		err := &RetryError{Err: want, RetryWait: 5 * time.Second, Delivered: 1, Failed: 2, Pending: 3}

		assert.Equal(t, "boom", err.Error())
		assert.ErrorIs(t, err.Unwrap(), want)
		assert.Equal(t, 5*time.Second, err.RetryAfter())

		delivered, failed, pending := err.NotificationStats()
		assert.Equal(t, 1, delivered)
		assert.Equal(t, 2, failed)
		assert.Equal(t, 3, pending)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("copies targets and initializes defaults", func(t *testing.T) {
		t.Parallel()

		targets := []target.Notifier{&recordingTarget{target: target.Target{Type: "webhook", Name: "ops"}}}

		fanout := New(targets)
		targets[0] = nil

		require.Len(t, fanout.targets, 1)
		require.NotNil(t, fanout.targets[0])
		assert.Equal(t, defaultInitialNotificationBackoff, fanout.InitialBackoff)
		assert.Equal(t, defaultMaxNotificationBackoff, fanout.MaxBackoff)
	})
}

func TestFanoutNextBackoff(t *testing.T) {
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

func TestFanoutConfiguredBackoff(t *testing.T) {
	t.Parallel()

	t.Run("uses configured initial and max values", func(t *testing.T) {
		t.Parallel()

		fanout := &Fanout{InitialBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second}

		assert.Equal(t, 5*time.Second, fanout.initialBackoffLocked())
		assert.Equal(t, 10*time.Second, fanout.maxBackoffLocked())
	})

	t.Run("uses defaults", func(t *testing.T) {
		t.Parallel()

		fanout := &Fanout{}

		assert.Equal(t, defaultInitialNotificationBackoff, fanout.initialBackoffLocked())
		assert.Equal(t, defaultMaxNotificationBackoff, fanout.maxBackoffLocked())
	})
}

func TestFanoutEventStateLocked(t *testing.T) {
	t.Parallel()

	t.Run("tracks target and attempts for one event", func(t *testing.T) {
		t.Parallel()

		fanout := New([]target.Notifier{&recordingTarget{target: target.Target{Type: "webhook", Name: "ops"}}})
		key := deliveryKey(testEvent().NotificationID)

		fanout.mu.Lock()
		state := fanout.eventStateLocked(key)
		state.delivered[0] = true
		state.attempts = 2
		fanout.targets = append(fanout.targets, &recordingTarget{target: target.Target{Type: "email", Name: "primary"}})
		state = fanout.eventStateLocked(key)
		fanout.mu.Unlock()

		require.Len(t, state.delivered, 2)
		assert.True(t, state.delivered[0])
		assert.False(t, state.delivered[1])
		assert.Equal(t, 2, state.attempts)
	})
}
