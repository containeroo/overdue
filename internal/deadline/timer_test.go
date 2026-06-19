package deadline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimer_Sync(t *testing.T) {
	t.Parallel()
	t.Run("arms timer when active", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		defer timer.Stop()

		timer.Sync(time.Now().Add(10*time.Millisecond), true)

		assert.NotNil(t, timer.timer)
		assert.NotNil(t, timer.C())
	})

	t.Run("stops timer when inactive", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		timer.Reset(time.Hour)

		timer.Sync(time.Time{}, false)

		assert.Nil(t, timer.timer)
		assert.Nil(t, timer.C())
	})
}

func TestTimer_ResetAt(t *testing.T) {
	t.Parallel()
	t.Run("fires at deadline", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		defer timer.Stop()

		timer.ResetAt(time.Now().Add(10 * time.Millisecond))

		select {
		case <-timer.C():
		case <-time.After(time.Second):
			require.Fail(t, "timer did not fire")
		}
	})
}

func TestTimer_Reset(t *testing.T) {
	t.Parallel()
	t.Run("allocates timer lazily", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		defer timer.Stop()

		timer.Reset(time.Hour)

		assert.NotNil(t, timer.timer)
		assert.NotNil(t, timer.C())
	})

	t.Run("resets existing timer", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		defer timer.Stop()

		timer.Reset(time.Hour)
		first := timer.timer

		timer.Reset(10 * time.Millisecond)

		assert.Same(t, first, timer.timer)

		select {
		case <-timer.C():
		case <-time.After(time.Second):
			require.Fail(t, "timer did not fire after reset")
		}
	})

	t.Run("normalizes negative duration to immediate fire", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		defer timer.Stop()

		timer.Reset(-time.Second)

		select {
		case <-timer.C():
		case <-time.After(time.Second):
			require.Fail(t, "timer did not fire")
		}
	})
}

func TestTimer_Stop(t *testing.T) {
	t.Parallel()
	t.Run("is safe when inactive", func(t *testing.T) {
		t.Parallel()
		var timer Timer

		timer.Stop()

		assert.Nil(t, timer.timer)
		assert.Nil(t, timer.C())
	})

	t.Run("clears active timer", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		timer.Reset(time.Hour)

		timer.Stop()

		assert.Nil(t, timer.timer)
		assert.Nil(t, timer.C())
	})
}

func TestTimer_C(t *testing.T) {
	t.Parallel()
	t.Run("returns nil when inactive", func(t *testing.T) {
		t.Parallel()
		var timer Timer

		assert.Nil(t, timer.C())
	})

	t.Run("returns timer channel when active", func(t *testing.T) {
		t.Parallel()
		var timer Timer
		defer timer.Stop()

		timer.Reset(time.Hour)

		assert.NotNil(t, timer.C())
	})
}
