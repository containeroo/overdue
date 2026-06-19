package app

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("serves http requests", func(t *testing.T) {
		// Do not call t.Parallel here. This test starts a real HTTP server.
		addr := freeTCPAddr(t)

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- Run(ctx, "dev", "none", []string{
				"--listen-address=" + addr,
				"--expected-every=10s",
				"--alerting-delay=2s",
			}, &stdout, &stderr, testTemplateFS())
		}()

		client := &http.Client{Timeout: time.Second}
		baseURL := "http://" + addr

		waitForHTTPStatus(t, client, baseURL+"/healthz", http.MethodGet, http.StatusOK)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/check-in", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() // nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Run did not stop after context cancellation")
		}

		assert.NotEmpty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("runs with valid config until context cancellation", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := Run(ctx, "dev", "none", []string{
			"--listen-address=:0",
			"--expected-every=10s",
			"--alerting-delay=2s",
		}, &stdout, &stderr, testTemplateFS())

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "starting overdue")
		assert.Contains(t, stdout.String(), "configured notifiers")
		assert.Empty(t, stderr.String())
	})

	t.Run("prints help", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		err := Run(context.Background(), "dev", "none", []string{"--help"}, &stdout, &stderr, testTemplateFS())

		require.NoError(t, err)
		assert.NotEmpty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("prints version", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		err := Run(context.Background(), "1.2.3", "abc123", []string{"--version"}, &stdout, &stderr, testTemplateFS())

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "1.2.3")
		assert.Empty(t, stderr.String())
	})

	t.Run("returns parse errors", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		err := Run(context.Background(), "dev", "none", []string{"--expected-every=0s", "--alerting-delay=1s"}, &stdout, &stderr, testTemplateFS())

		require.Error(t, err)
		assert.Empty(t, stdout.String())
		assert.NotEmpty(t, stderr.String())
		assert.Contains(t, stderr.String(), "expected-every")
	})

	t.Run("returns template validation errors", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		err := Run(context.Background(), "dev", "none", []string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.template=builtin:missing",
		}, &stdout, &stderr, testTemplateFS())

		require.Error(t, err)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
		assert.NotEmpty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("returns notifier setup errors", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		templateFS := &failAfterFirstOpenFS{
			FS:       testTemplateFS(),
			filename: "slack-incoming-webhook.tmpl",
		}

		err := Run(context.Background(), "dev", "none", []string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.template=builtin:slack-incoming-webhook",
		}, &stdout, &stderr, templateFS)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `read built-in template "slack-incoming-webhook"`)
		assert.NotEmpty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("runs start active until context cancellation", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := Run(ctx, "dev", "none", []string{
			"--listen-address=:0",
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--start-active",
		}, &stdout, &stderr, testTemplateFS())

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "check-in monitor activated at startup")
		assert.Empty(t, stderr.String())
	})

	t.Run("returns server run errors", func(t *testing.T) {
		t.Parallel()

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close() // nolint:errcheck

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		err = Run(context.Background(), "dev", "none", []string{
			"--listen-address=" + listener.Addr().String(),
			"--expected-every=10s",
			"--alerting-delay=2s",
		}, &stdout, &stderr, testTemplateFS())

		require.Error(t, err)
		assert.NotEmpty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})
}

// failAfterFirstOpenFS behaves like FS until filename has been opened once.
type failAfterFirstOpenFS struct {
	mu       sync.Mutex
	FS       fs.FS
	filename string
	opened   int
}

// Open opens name unless the configured file was already opened once.
func (f *failAfterFirstOpenFS) Open(name string) (fs.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if name == f.filename {
		f.opened++
		if f.opened > 1 {
			return nil, fs.ErrNotExist
		}
	}

	return f.FS.Open(name)
}

// testTemplateFS returns built-in template fixtures.
func testTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"email-html.tmpl": {
			Data: []byte(`{{ .Title }}`),
		},
		"slack-incoming-webhook.tmpl": {
			Data: []byte(`{"text":{{ json .Text }}}`),
		},
		"slack-chat-post-message.tmpl": {
			Data: []byte(`{"channel":"#alertmanager","text":{{ json .Text }}}`),
		},
	}
}

// freeTCPAddr returns an available local TCP address for integration smoke tests.
func freeTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	return addr
}

// waitForHTTPStatus waits until the server responds with the expected status.
func waitForHTTPStatus(t *testing.T, client *http.Client, url, method string, wantStatus int) {
	t.Helper()

	require.Eventually(t, func() bool {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return false
		}

		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close() // nolint:errcheck

		return resp.StatusCode == wantStatus
	}, 2*time.Second, 10*time.Millisecond, fmt.Sprintf("%s %s did not return %d", method, url, wantStatus))
}
