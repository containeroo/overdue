package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("builds request origin metadata", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "https://overdue.example.test/checkin?token=secret", nil)
		req.RemoteAddr = "10.42.1.17:51234"
		req.Header.Set("User-Agent", "curl/8.7.1")
		req.Header.Set("X-Request-ID", "request-1")
		req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.42.0.5")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "overdue.example.test")
		req.Header.Set("X-Real-IP", "203.0.113.11")

		got := requestMetadata(req)

		assert.Equal(t, "10.42.1.17:51234", got.Remote)
		assert.Equal(t, "203.0.113.10", got.ClientIP)
		assert.Equal(t, http.MethodPost, got.Method)
		assert.Equal(t, "/checkin", got.Path)
		assert.Equal(t, "overdue.example.test", got.Host)
		assert.Equal(t, "HTTP/1.1", got.Proto)
		assert.Equal(t, "curl/8.7.1", got.UserAgent)
		assert.Equal(t, "request-1", got.RequestID)
		assert.Equal(t, "203.0.113.10, 10.42.0.5", got.XForwardedFor)
		assert.Equal(t, "https", got.XForwardedProto)
		assert.Equal(t, "overdue.example.test", got.XForwardedHost)
		assert.Equal(t, "203.0.113.11", got.XRealIP)
	})

	t.Run("falls back to standardized forwarded header", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		req.RemoteAddr = "10.42.1.17:51234"
		req.Header.Set("Forwarded", `for="203.0.113.20";proto=https;host=overdue.example.test`)

		got := requestMetadata(req)

		assert.Equal(t, "203.0.113.20", got.ClientIP)
		assert.Equal(t, `for="203.0.113.20";proto=https;host=overdue.example.test`, got.Forwarded)
	})

	t.Run("falls back to remote host", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		req.RemoteAddr = "[::1]:51234"

		got := requestMetadata(req)

		assert.Equal(t, "::1", got.ClientIP)
		assert.Equal(t, "[::1]:51234", got.Remote)
	})

	t.Run("handles nil request", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, requestMetadata(nil).LogFields())
	})
}

func TestHeaderValue(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "https://overdue.example.test/checkin?password=secret", nil)
	request.Header.Set("token", "secret")

	t.Run("returns trimmed value", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "secret", headerValue(request, "token"))
	})

	t.Run("returns empty string for missing header", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, headerValue(request, "password"))
	})
}

func TestRequestMetadata_LogFields(t *testing.T) {
	t.Parallel()

	t.Run("builds structured log fields", func(t *testing.T) {
		t.Parallel()

		fields := RequestMetadata{
			Remote:          "10.42.1.17:51234",
			ClientIP:        "203.0.113.10",
			Method:          "POST",
			Path:            "/checkin",
			Host:            "overdue.example.test",
			Proto:           "HTTP/1.1",
			UserAgent:       "curl/8.7.1",
			RequestID:       "request-1",
			XForwardedFor:   "203.0.113.10, 10.42.0.5",
			XForwardedProto: "https",
			XForwardedHost:  "overdue.example.test",
			XRealIP:         "203.0.113.11",
			Forwarded:       `for="203.0.113.20";proto=https;host=overdue.example.test`,
		}.LogFields()

		got := logFieldsMap(fields)

		assert.Equal(t, "10.42.1.17:51234", got["remote"])
		assert.Equal(t, "203.0.113.10", got["clientIP"])
		assert.Equal(t, "POST", got["method"])
		assert.Equal(t, "/checkin", got["path"])
		assert.Equal(t, "overdue.example.test", got["host"])
		assert.Equal(t, "HTTP/1.1", got["proto"])
		assert.Equal(t, "curl/8.7.1", got["userAgent"])
		assert.Equal(t, "request-1", got["requestID"])
		assert.Equal(t, "203.0.113.10, 10.42.0.5", got["xForwardedFor"])
		assert.Equal(t, "https", got["xForwardedProto"])
		assert.Equal(t, "overdue.example.test", got["xForwardedHost"])
		assert.Equal(t, "203.0.113.11", got["xRealIP"])
		assert.Equal(t, `for="203.0.113.20";proto=https;host=overdue.example.test`, got["forwarded"])
	})

	t.Run("omits blank fields", func(t *testing.T) {
		t.Parallel()

		fields := RequestMetadata{
			Remote: "10.42.1.17:51234",
			Path:   " ",
		}.LogFields()

		got := logFieldsMap(fields)

		assert.Equal(t, map[string]any{"remote": "10.42.1.17:51234"}, got)
	})
}

func TestWantsDetails(t *testing.T) {
	t.Parallel()
	t.Run("accepts true", func(t *testing.T) {
		t.Parallel()

		assert.True(t, wantsDetails(httptest.NewRequest(http.MethodPost, "/checkin?details=true", nil)))
	})

	t.Run("accepts one", func(t *testing.T) {
		t.Parallel()

		assert.True(t, wantsDetails(httptest.NewRequest(http.MethodPost, "/checkin?details=1", nil)))
	})

	t.Run("accepts yes", func(t *testing.T) {
		t.Parallel()

		assert.True(t, wantsDetails(httptest.NewRequest(http.MethodPost, "/checkin?details=yes", nil)))
	})

	t.Run("rejects false", func(t *testing.T) {
		t.Parallel()

		assert.False(t, wantsDetails(httptest.NewRequest(http.MethodPost, "/checkin?details=false", nil)))
	})

	t.Run("rejects missing value", func(t *testing.T) {
		t.Parallel()

		assert.False(t, wantsDetails(httptest.NewRequest(http.MethodPost, "/checkin", nil)))
	})
}

func TestRequestPath(t *testing.T) {
	t.Parallel()

	t.Run("returns escaped path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/check%20in?token=secret", nil)

		assert.Equal(t, "/check%20in", requestPath(req))
	})

	t.Run("returns empty path without url", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, requestPath(&http.Request{}))
	})
}

func TestBestEffortClientIP(t *testing.T) {
	t.Parallel()

	t.Run("prefers x forwarded for", func(t *testing.T) {
		t.Parallel()

		got := bestEffortClientIP("10.42.1.17:51234", "203.0.113.10", `for="203.0.113.20"`, "203.0.113.11")

		assert.Equal(t, "203.0.113.10", got)
	})

	t.Run("falls back to forwarded", func(t *testing.T) {
		t.Parallel()

		got := bestEffortClientIP("10.42.1.17:51234", "", `for="203.0.113.20"`, "203.0.113.11")

		assert.Equal(t, "203.0.113.20", got)
	})

	t.Run("falls back to x real ip", func(t *testing.T) {
		t.Parallel()

		got := bestEffortClientIP("10.42.1.17:51234", "", "", "203.0.113.11")

		assert.Equal(t, "203.0.113.11", got)
	})

	t.Run("falls back to remote", func(t *testing.T) {
		t.Parallel()

		got := bestEffortClientIP("10.42.1.17:51234", "", "", "")

		assert.Equal(t, "10.42.1.17", got)
	})
}

func TestFirstForwardedFor(t *testing.T) {
	t.Parallel()

	t.Run("returns first forwarded address", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "203.0.113.10", firstForwardedFor("203.0.113.10, 10.42.0.5"))
	})
}

func TestForwardedForParam(t *testing.T) {
	t.Parallel()

	t.Run("extracts for parameter", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "203.0.113.20", forwardedForParam(`for="203.0.113.20";proto=https`))
	})

	t.Run("ignores unknown value", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, forwardedForParam(`for=unknown;proto=https`))
	})
}

func TestHostOnly(t *testing.T) {
	t.Parallel()

	t.Run("strips port", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "10.42.1.17", hostOnly("10.42.1.17:51234"))
	})

	t.Run("normalizes ipv6 brackets", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "::1", hostOnly("[::1]:51234"))
	})

	t.Run("returns non ip host", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "example.test", hostOnly("example.test"))
	})
}

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	t.Run("returns first non blank value", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "request-1", firstNonEmpty("", " ", "request-1", "request-2"))
	})

	t.Run("returns empty string when all values are blank", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, firstNonEmpty("", " "))
	})
}

func TestAppendNonEmptyLogField(t *testing.T) {
	t.Parallel()

	t.Run("appends non blank value", func(t *testing.T) {
		t.Parallel()

		fields := appendNonEmptyLogField(nil, "remote", " 10.42.1.17:51234 ")

		assert.Equal(t, []any{"remote", "10.42.1.17:51234"}, fields)
	})

	t.Run("skips blank value", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, appendNonEmptyLogField(nil, "remote", " "))
	})
}

// logFieldsMap converts alternating slog fields into a map for tests.
func logFieldsMap(fields []any) map[string]any {
	out := make(map[string]any, len(fields)/2)

	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		out[key] = fields[i+1]
	}

	return out
}
