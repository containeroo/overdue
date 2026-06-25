package webhook

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("builds webhook with defensive config copies and defaults", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{"Authorization": "Bearer token"}
		customData := map[string]string{"channel": "#ops"}
		webhook := New(Config{
			Name:              "ops",
			URL:               "https://example.test/webhook",
			Headers:           headers,
			ContentTemplates:  render.ContentTemplates{CustomData: customData},
			Timeout:           5 * time.Second,
			ResponseBodyLimit: 128,
		}, Renderer{}, testLogger())
		headers["Authorization"] = "Bearer changed"
		customData["channel"] = "#changed"

		cfg := webhook.Config()
		assert.Equal(t, "Bearer token", cfg.Headers["Authorization"])
		assert.Equal(t, "#ops", cfg.ContentTemplates.CustomData["channel"])
		assert.Equal(t, http.MethodPost, cfg.Method)
		assert.Equal(t, LogResponseSummary, cfg.LogResponse)
		require.NotNil(t, webhook.Client())
		assert.Equal(t, 5*time.Second, webhook.Client().Timeout)
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "webhook logger must not be nil", func() {
			_ = New(Config{Timeout: time.Second}, Renderer{}, nil)
		})
	})

	t.Run("panics without positive timeout", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "webhook timeout must be greater than zero", func() {
			_ = New(Config{}, Renderer{}, testLogger())
		})
	})

	t.Run("panics with negative response body limit", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "webhook response body limit must not be negative", func() {
			_ = New(Config{Timeout: time.Second, ResponseBodyLimit: -1}, Renderer{}, testLogger())
		})
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()
	t.Run("returns defensive copy", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{
			Headers:           map[string]string{"Authorization": "Bearer token"},
			Timeout:           time.Second,
			ResponseBodyLimit: 128,
		}, Renderer{}, testLogger())

		cfg := webhook.Config()
		cfg.Headers["Authorization"] = "Bearer changed"

		assert.Equal(t, "Bearer token", webhook.Config().Headers["Authorization"])
	})
}

func TestNotifierClient(t *testing.T) {
	t.Parallel()
	t.Run("returns configured client", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Timeout: 3 * time.Second}, Renderer{}, testLogger())

		assert.Same(t, webhook.client, webhook.Client())
	})
}

func TestNotifierTarget(t *testing.T) {
	t.Parallel()

	t.Run("returns target metadata", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Name: "ops", Timeout: time.Second}, Renderer{}, testLogger())

		assert.Equal(t, target.Target{Type: "webhook", Name: "ops"}, webhook.Target())
	})
}

func TestNotifierNotify(t *testing.T) {
	t.Parallel()
	t.Run("posts rendered json request", func(t *testing.T) {
		t.Parallel()

		var called atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json; charset=utf-8", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.JSONEq(t, `{"title":"alerting prometheus","text":"prometheus is down"}`, string(body))

			w.Header().Set("X-Request-ID", "request-1")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		webhook := newTestNotifier(t, server.URL, LogResponseFull, 128)

		err := webhook.Notify(context.Background(), testEvent())

		require.NoError(t, err)
		assert.Equal(t, int32(1), called.Load())
	})

	t.Run("uses configured http method", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		webhook := newTestNotifier(t, server.URL, LogResponseSummary, 128)
		webhook.cfg.Method = http.MethodPatch

		err := webhook.Notify(context.Background(), testEvent())

		require.NoError(t, err)
	})

	t.Run("skips resolved events when disabled", func(t *testing.T) {
		t.Parallel()

		var called atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called.Add(1)
		}))
		defer server.Close()

		webhook := newTestNotifier(t, server.URL, LogResponseSummary, 128)
		webhook.cfg.SendResolved = false

		err := webhook.Notify(context.Background(), testResolvedEvent())

		require.ErrorIs(t, err, target.ErrSkipped)
		assert.Equal(t, int32(0), called.Load())
	})

	t.Run("returns renderer errors before request", func(t *testing.T) {
		t.Parallel()

		var called atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called.Add(1)
		}))
		defer server.Close()

		renderer, err := NewRenderer(nil, writeTemplate(t, `not json`), render.DefaultContentTemplates())
		require.NoError(t, err)
		webhook := New(Config{
			URL:               server.URL,
			Timeout:           time.Second,
			SendResolved:      true,
			ResponseBodyLimit: 128,
		}, renderer, testLogger())

		err = webhook.Notify(context.Background(), testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "result is not valid JSON")
		assert.Equal(t, int32(0), called.Load())
	})

	t.Run("returns request errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer server.Close()
		webhook := newTestNotifier(t, server.URL, LogResponseSummary, 128)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := webhook.Notify(ctx, testEvent())

		require.Error(t, err)
	})

	t.Run("returns response body read errors", func(t *testing.T) {
		t.Parallel()

		webhook := newTestNotifier(t, "http://example.test", LogResponseSummary, 128)
		webhook.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(errReader{err: errors.New("read failed")}),
			}, nil
		})}

		err := webhook.Notify(context.Background(), testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read response body")
	})

	t.Run("returns non success responses with truncated body", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "very long response body", http.StatusBadGateway)
		}))
		defer server.Close()

		webhook := newTestNotifier(t, server.URL, LogResponseBody, 4)

		err := webhook.Notify(context.Background(), testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "502 Bad Gateway")
		assert.Contains(t, err.Error(), "...(truncated)")
	})
}

func TestNotifierLogSuccessfulResponse(t *testing.T) {
	t.Parallel()
	t.Run("supports all logging modes", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Header: http.Header{"X-Test": []string{"yes"}}}
		for _, mode := range []LogResponse{LogResponseNone, LogResponseSummary, LogResponseBody, LogResponseFull, "unknown"} {
			webhook := New(Config{Timeout: time.Second, LogResponse: mode}, Renderer{}, testLogger())

			webhook.logSuccessfulResponse(testEvent(), resp, `{"ok":true}`, false, 10*time.Millisecond)
		}
	})
}

func TestNotifierResponseLogFields(t *testing.T) {
	t.Parallel()
	t.Run("builds summary fields", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Timeout: time.Second}, Renderer{}, testLogger())
		resp := &http.Response{Status: "200 OK", StatusCode: http.StatusOK}

		fields := webhook.responseLogFields(resp, "", false, 10*time.Millisecond, LogResponseSummary)

		assert.Contains(t, fields, "status")
		assert.Contains(t, fields, "statusCode")
		assert.NotContains(t, fields, "responseBody")
	})

	t.Run("includes parsed body for body mode", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Timeout: time.Second}, Renderer{}, testLogger())
		resp := &http.Response{Status: "200 OK", StatusCode: http.StatusOK}

		fields := webhook.responseLogFields(resp, `{"ok":true}`, true, 10*time.Millisecond, LogResponseBody)

		assert.Contains(t, fields, "responseBody")
		assert.Contains(t, fields, "bodyTruncated")
	})

	t.Run("includes headers for full mode", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Timeout: time.Second}, Renderer{}, testLogger())
		resp := &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Header: http.Header{"X-Test": []string{"yes"}}}

		fields := webhook.responseLogFields(resp, `{"ok":true}`, false, 10*time.Millisecond, LogResponseFull)

		assert.Contains(t, fields, "responseHeaders")
	})
}

func TestNotifierResponseError(t *testing.T) {
	t.Parallel()
	t.Run("uses generic label without name", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Timeout: time.Second}, Renderer{}, testLogger())
		resp := &http.Response{Status: "500 Internal Server Error"}

		err := webhook.responseError(resp, "", false)

		require.Error(t, err)
		assert.Equal(t, "webhook returned 500 Internal Server Error", err.Error())
	})

	t.Run("includes named label and body", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Name: "ops", Timeout: time.Second}, Renderer{}, testLogger())
		resp := &http.Response{Status: "500 Internal Server Error"}

		err := webhook.responseError(resp, "failed", true)

		require.Error(t, err)
		assert.Equal(t, `webhook "ops" returned 500 Internal Server Error: failed...(truncated)`, err.Error())
	})
}

func TestReadNotifierResponseBody(t *testing.T) {
	t.Parallel()
	t.Run("reads and trims response body", func(t *testing.T) {
		t.Parallel()

		body, truncated, err := readNotifierResponseBody(strings.NewReader(" ok \n"), 10)

		require.NoError(t, err)
		assert.Equal(t, "ok", body)
		assert.False(t, truncated)
	})

	t.Run("truncates response body", func(t *testing.T) {
		t.Parallel()

		body, truncated, err := readNotifierResponseBody(strings.NewReader("abcdef"), 3)

		require.NoError(t, err)
		assert.Equal(t, "abc", body)
		assert.True(t, truncated)
	})

	t.Run("rejects negative limit", func(t *testing.T) {
		t.Parallel()

		_, _, err := readNotifierResponseBody(strings.NewReader("abc"), -1)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be negative")
	})

	t.Run("returns read errors", func(t *testing.T) {
		t.Parallel()

		_, _, err := readNotifierResponseBody(errReader{err: errors.New("boom")}, 10)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}

func TestNotifierResponseBodyValue(t *testing.T) {
	t.Parallel()
	t.Run("returns nil for empty body", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, webhookResponseBodyValue(""))
	})

	t.Run("returns structured json values", func(t *testing.T) {
		t.Parallel()

		value := webhookResponseBodyValue(`{"ok":true}`)

		assert.Equal(t, map[string]any{"ok": true}, value)
	})

	t.Run("returns plain text for non json body", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "plain text", webhookResponseBodyValue("plain text"))
	})
}

func TestCloneResponseHeaders(t *testing.T) {
	t.Parallel()
	t.Run("returns nil for empty headers", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, cloneResponseHeaders(nil))
	})

	t.Run("returns defensive copy", func(t *testing.T) {
		t.Parallel()

		headers := http.Header{"X-Test": []string{"yes"}}

		clone := cloneResponseHeaders(headers)
		clone["X-Test"][0] = "changed"

		assert.Equal(t, "yes", headers.Get("X-Test"))
	})
}

func TestNotifierClientConfig(t *testing.T) {
	t.Parallel()
	t.Run("uses timeout and default tls verification", func(t *testing.T) {
		t.Parallel()

		client := webhookClient(5*time.Second, false)

		require.NotNil(t, client)
		assert.Equal(t, 5*time.Second, client.Timeout)
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.False(t, transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify)
	})

	t.Run("can skip tls verification", func(t *testing.T) {
		t.Parallel()

		client := webhookClient(5*time.Second, true)

		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	})
}

func TestNotifierLabel(t *testing.T) {
	t.Parallel()
	t.Run("returns generic label without name", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Timeout: time.Second}, Renderer{}, testLogger())

		assert.Equal(t, "webhook", webhook.label())
	})

	t.Run("returns named label", func(t *testing.T) {
		t.Parallel()

		webhook := New(Config{Name: "ops", Timeout: time.Second}, Renderer{}, testLogger())

		assert.Equal(t, `webhook "ops"`, webhook.label())
	})
}

// roundTripFunc adapts a function to http.RoundTripper for deterministic client errors.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// newTestNotifier builds a webhook using deterministic templates.
func newTestNotifier(t *testing.T, url string, logResponse LogResponse, responseBodyLimit int) Notifier {
	t.Helper()

	renderer, err := NewRenderer(testTemplateFS(), "builtin:webhook", testContentTemplates())
	require.NoError(t, err)

	return New(Config{
		Name:              "ops",
		URL:               url,
		Headers:           map[string]string{"Authorization": "Bearer token"},
		Timeout:           time.Second,
		SendResolved:      true,
		ResponseBodyLimit: responseBodyLimit,
		LogResponse:       logResponse,
	}, renderer, testLogger())
}
