package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateKeyValue(t *testing.T) {
	t.Parallel()

	t.Run("accepts key value", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateKeyValue("owner=platform"))
	})

	t.Run("rejects missing separator", func(t *testing.T) {
		t.Parallel()

		require.Error(t, ValidateKeyValue("owner"))
	})

	t.Run("rejects empty key", func(t *testing.T) {
		t.Parallel()

		require.Error(t, ValidateKeyValue(" =platform"))
	})
}

func TestHeaderMap(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for empty headers", func(t *testing.T) {
		t.Parallel()

		headers, err := headerMap("webhook", "ops", nil)

		require.NoError(t, err)
		assert.Nil(t, headers)
	})

	t.Run("parses headers", func(t *testing.T) {
		t.Parallel()

		headers, err := headerMap("webhook", "ops", []string{"Authorization=Bearer token", "X-Test=yes"})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"Authorization": "Bearer token", "X-Test": "yes"}, headers)
	})

	t.Run("rejects invalid header", func(t *testing.T) {
		t.Parallel()

		_, err := headerMap("webhook", "ops", []string{"invalid"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.headers"`)
	})
}

func TestKeyValueMap(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for empty values", func(t *testing.T) {
		t.Parallel()

		values, err := keyValueMap("webhook", "ops", "custom-data", nil)

		require.NoError(t, err)
		assert.Nil(t, values)
	})

	t.Run("parses key value pairs", func(t *testing.T) {
		t.Parallel()

		values, err := keyValueMap("webhook", "ops", "custom-data", []string{"channel=#ops", "owner=platform"})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"channel": "#ops", "owner": "platform"}, values)
	})

	t.Run("rejects missing separator", func(t *testing.T) {
		t.Parallel()

		_, err := keyValueMap("webhook", "ops", "custom-data", []string{"invalid"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.custom-data"`)
	})

	t.Run("rejects empty keys", func(t *testing.T) {
		t.Parallel()

		_, err := keyValueMap("email", "ops", "custom-data", []string{" =value"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--email.ops.custom-data"`)
	})
}
