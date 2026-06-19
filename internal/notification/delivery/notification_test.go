package delivery

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
	t.Run("is a reusable sentinel error", func(t *testing.T) {
		t.Parallel()

		err := fmt.Errorf("target skipped delivery: %w", ErrSkipped)

		assert.Equal(t, "notification skipped", ErrSkipped.Error())
		assert.ErrorIs(t, err, ErrSkipped)
	})
}

func TestNotifier(t *testing.T) {
	t.Parallel()
	t.Run("sends lifecycle events", func(t *testing.T) {
		t.Parallel()

		want := monitor.Event{NotificationID: "notification-1"}
		notifier := notifierFunc(func(_ context.Context, event monitor.Event) error {
			assert.Equal(t, want, event)
			return nil
		})

		err := notifier.Notify(context.Background(), want)

		require.NoError(t, err)
	})

	t.Run("returns notify errors", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		notifier := notifierFunc(func(context.Context, monitor.Event) error {
			return wantErr
		})

		err := notifier.Notify(context.Background(), monitor.Event{})

		require.ErrorIs(t, err, wantErr)
	})
}

func TestTargeter(t *testing.T) {
	t.Parallel()
	t.Run("returns notification target metadata", func(t *testing.T) {
		t.Parallel()

		provider := targetProvider{target: Target{Type: "webhook", Name: "ops"}}

		assert.Equal(t, Target{Type: "webhook", Name: "ops"}, provider.NotificationTarget())
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

type notifierFunc func(context.Context, monitor.Event) error

func (f notifierFunc) Notify(ctx context.Context, event monitor.Event) error {
	return f(ctx, event)
}

type targetProvider struct {
	target Target
}

func (p targetProvider) NotificationTarget() Target {
	return p.target
}

type statusProvider struct {
	status Status
}

func (p statusProvider) NotificationStatus() Status {
	return p.status
}
