package target

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeliveryStatus(t *testing.T) {
	t.Parallel()

	t.Run("defines stable wire values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "idle", string(StatusIdle))
		assert.Equal(t, "pending", string(StatusPending))
		assert.Equal(t, "delivered", string(StatusDelivered))
		assert.Equal(t, "failed", string(StatusFailed))
		assert.Equal(t, "skipped", string(StatusSkipped))
		assert.Equal(t, "partial_failure", string(StatusPartialFailure))
	})
}

func TestErrSkipped(t *testing.T) {
	t.Parallel()

	t.Run("can be wrapped and matched", func(t *testing.T) {
		t.Parallel()

		err := fmt.Errorf("notification delivery skipped: %w", ErrSkipped)

		assert.Equal(t, "notification skipped", ErrSkipped.Error())
		assert.ErrorIs(t, err, ErrSkipped)
	})
}

func TestDispatcher(t *testing.T) {
	t.Parallel()

	t.Run("sends lifecycle events", func(t *testing.T) {
		t.Parallel()

		want := monitor.Event{NotificationID: "notification-1"}
		dispatcher := dispatcherFunc(func(_ context.Context, event monitor.Event) error {
			assert.Equal(t, want, event)
			return nil
		})

		err := dispatcher.Notify(context.Background(), want)

		require.NoError(t, err)
	})

	t.Run("returns notify errors", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		dispatcher := dispatcherFunc(func(context.Context, monitor.Event) error {
			return wantErr
		})

		err := dispatcher.Notify(context.Background(), monitor.Event{})

		require.ErrorIs(t, err, wantErr)
	})
}

func TestNotifier(t *testing.T) {
	t.Parallel()

	t.Run("combines delivery with target metadata", func(t *testing.T) {
		t.Parallel()

		wantEvent := monitor.Event{NotificationID: "notification-1"}
		notifier := namedNotifier{
			dispatcherFunc: dispatcherFunc(func(_ context.Context, event monitor.Event) error {
				assert.Equal(t, wantEvent, event)
				return nil
			}),
			target: Target{Type: "webhook", Name: "ops"},
		}

		var targetNotifier Notifier = notifier

		require.NoError(t, targetNotifier.Notify(context.Background(), wantEvent))
		assert.Equal(t, Target{Type: "webhook", Name: "ops"}, targetNotifier.Target())
	})
}

func TestStatusProvider(t *testing.T) {
	t.Parallel()

	t.Run("returns aggregate notification status", func(t *testing.T) {
		t.Parallel()

		lastAttemptAt := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		lastDeliveredAt := lastAttemptAt.Add(time.Second)
		want := Status{
			Status:    StatusDelivered,
			Total:     1,
			Delivered: 1,
			Targets: []TargetStatus{{
				Type:            "email",
				Name:            "primary",
				Status:          StatusDelivered,
				LastAttemptAt:   &lastAttemptAt,
				LastDeliveredAt: &lastDeliveredAt,
			}},
		}
		provider := statusProvider{status: want}

		assert.Equal(t, want, provider.NotificationStatus())
	})
}

type dispatcherFunc func(context.Context, monitor.Event) error

func (f dispatcherFunc) Notify(ctx context.Context, event monitor.Event) error {
	return f(ctx, event)
}

type namedNotifier struct {
	dispatcherFunc
	target Target
}

func (n namedNotifier) Target() Target {
	return n.target
}

type statusProvider struct {
	status Status
}

func (p statusProvider) NotificationStatus() Status {
	return p.status
}
