package flag

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateListenAddress(t *testing.T) {
	t.Parallel()

	t.Run("accepts default listen address", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateListenAddress(&net.TCPAddr{IP: nil, Port: 8080}))
	})

	t.Run("accepts concrete ip and port", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateListenAddress(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}))
	})

	t.Run("rejects nil address", func(t *testing.T) {
		t.Parallel()

		require.EqualError(t, validateListenAddress(nil), "listen-address must not be empty")
	})

	t.Run("rejects concrete ip without port", func(t *testing.T) {
		t.Parallel()

		require.EqualError(t, validateListenAddress(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}), "listen-address must not be empty")
	})
}

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

		require.EqualError(t, validatePublicURL("overdue"), "public-url must be a valid absolute URL")
	})
}

func TestValidateWebhookURL(t *testing.T) {
	t.Parallel()

	t.Run("accepts empty url", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateWebhookURL(""))
	})

	t.Run("accepts absolute url", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateWebhookURL("https://example.test/webhook"))
	})

	t.Run("rejects relative url", func(t *testing.T) {
		t.Parallel()

		require.EqualError(t, validateWebhookURL("webhook"), "must be a valid URL")
	})
}

func TestValidateCheckInName(t *testing.T) {
	t.Parallel()

	t.Run("accepts non-empty name", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateCheckInName("default"))
	})

	t.Run("rejects empty name", func(t *testing.T) {
		t.Parallel()

		require.EqualError(t, validateCheckInName(" "), "name must not be empty")
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

	t.Run("accepts non-root path", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validatePath("/checkin"))
	})

	t.Run("rejects root path", func(t *testing.T) {
		t.Parallel()

		require.EqualError(t, validatePath("/"), "path must be a non-root route")
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

		require.NoError(t, validateAuthToken(strings.Repeat("a", config.MinAuthTokenLength)))
	})

	t.Run("rejects short token", func(t *testing.T) {
		t.Parallel()

		require.EqualError(
			t,
			validateAuthToken("short"),
			"auth-token must be at least 32 characters",
		)
	})

	t.Run("rejects long token", func(t *testing.T) {
		t.Parallel()

		require.EqualError(
			t,
			validateAuthToken(strings.Repeat("a", config.MaxAuthTokenLength+1)),
			"auth-token must be at most 4096 characters",
		)
	})

	t.Run("rejects whitespace", func(t *testing.T) {
		t.Parallel()

		err := validateAuthToken(strings.Repeat("a", config.MinAuthTokenLength-1) + " ")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth-token must contain printable ASCII characters only")
	})
}

func TestValidateHeader(t *testing.T) {
	t.Parallel()

	t.Run("accepts key value header", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateHeader("X-Test=yes"))
	})

	t.Run("rejects invalid header", func(t *testing.T) {
		t.Parallel()

		require.Error(t, validateHeader("invalid"))
	})
}

func TestIntAtLeast(t *testing.T) {
	t.Parallel()

	t.Run("accepts minimum", func(t *testing.T) {
		t.Parallel()

		validate := intAtLeast(1)

		require.NoError(t, validate(1))
	})

	t.Run("accepts greater value", func(t *testing.T) {
		t.Parallel()

		validate := intAtLeast(1)

		require.NoError(t, validate(42))
	})

	t.Run("rejects lower value", func(t *testing.T) {
		t.Parallel()

		validate := intAtLeast(1)

		require.EqualError(t, validate(0), "must be at least 1")
	})
}

func TestDurationGreaterThanZero(t *testing.T) {
	t.Parallel()

	t.Run("accepts positive duration", func(t *testing.T) {
		t.Parallel()

		validate := durationGreaterThanZero()

		require.NoError(t, validate(time.Nanosecond))
	})

	t.Run("rejects zero duration", func(t *testing.T) {
		t.Parallel()

		validate := durationGreaterThanZero()

		require.EqualError(t, validate(0), "must be greater than zero")
	})

	t.Run("rejects negative duration", func(t *testing.T) {
		t.Parallel()

		validate := durationGreaterThanZero()

		require.EqualError(t, validate(-time.Second), "must be greater than zero")
	})
}
