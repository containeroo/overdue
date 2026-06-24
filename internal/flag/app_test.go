package flag

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePublicURL(t *testing.T) {
	t.Parallel()

	t.Run("trims spaces and trailing slashes", func(t *testing.T) {
		t.Parallel()

		got := normalizePublicURL(" https://overdue.example.test/overdue/ ")

		assert.Equal(t, "https://overdue.example.test/overdue", got)
	})
}

func TestValidatePublicURL(t *testing.T) {
	t.Parallel()

	t.Run("accepts empty public url", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validatePublicURL(""))
	})

	t.Run("accepts absolute url", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validatePublicURL("https://overdue.example.test/overdue"))
	})

	t.Run("rejects relative url", func(t *testing.T) {
		t.Parallel()

		require.Error(t, validatePublicURL("overdue"))
	})
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	t.Run("defaults empty path", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/checkin", normalizePath(" "))
	})

	t.Run("adds leading slash and removes trailing slash", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/custom-check-in", normalizePath(" custom-check-in/ "))
	})
}

func TestValidatePath(t *testing.T) {
	t.Parallel()

	t.Run("accepts non root path", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validatePath("/checkin"))
	})

	t.Run("rejects root path", func(t *testing.T) {
		t.Parallel()

		require.Error(t, validatePath("/"))
	})
}

func TestValidateAuthToken(t *testing.T) {
	t.Parallel()

	t.Run("accepts empty token", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateAuthToken(""))
	})

	t.Run("accepts printable ascii token", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateAuthToken(strings.Repeat("a", minAuthTokenLength)))
	})

	t.Run("rejects short token", func(t *testing.T) {
		t.Parallel()

		require.Error(t, validateAuthToken("short"))
	})

	t.Run("rejects whitespace", func(t *testing.T) {
		t.Parallel()

		require.Error(t, validateAuthToken(strings.Repeat("a", minAuthTokenLength-1)+" "))
	})
}
