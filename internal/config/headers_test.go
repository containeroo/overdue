package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderMap(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for empty headers", func(t *testing.T) {
		t.Parallel()

		headers, err := headerMap("webhook", "ops", nil)

		require.NoError(t, err)
		assert.Nil(t, headers)
	})

	t.Run("parses repeated header values", func(t *testing.T) {
		t.Parallel()

		headers, err := headerMap("webhook", "ops", []string{
			"Authorization=Bearer token",
			"X-Team=platform",
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"Authorization": "Bearer token",
			"X-Team":        "platform",
		}, headers)
	})

	t.Run("parses comma separated header values", func(t *testing.T) {
		t.Parallel()

		headers, err := headerMap("webhook", "ops", []string{
			"Authorization=Bearer token, X-Team=platform",
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"Authorization": "Bearer token",
			"X-Team":        "platform",
		}, headers)
	})

	t.Run("rejects invalid header values", func(t *testing.T) {
		t.Parallel()

		_, err := headerMap("webhook", "ops", []string{"invalid"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.headers"`)
	})

	t.Run("rejects duplicate headers across flag values", func(t *testing.T) {
		t.Parallel()

		_, err := headerMap("webhook", "ops", []string{
			"X-Team=platform",
			"X-Team=ops",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.headers"`)
		assert.Contains(t, err.Error(), "duplicate header key found: X-Team")
	})
}
