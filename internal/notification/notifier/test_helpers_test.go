package notifier

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTemplate writes a temporary template.
func writeTemplate(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// testTemplateFS returns built-in template fixtures.
func testTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"email-html.tmpl": {
			Data: []byte(`{{ .Title }} Check-in status: {{ .Status }}`),
		},
		"slack-incoming-webhook.tmpl": {
			Data: []byte(`{"attachments":[],"text":{{ json .Text }}}`),
		},
		"slack-chat-post-message.tmpl": {
			Data: []byte(`{"channel":"#alertmanager","text":{{ json .Text }}}`),
		},
	}
}

// assertWebhookClient checks webhook client settings.
func assertWebhookClient(t *testing.T, client *http.Client, wantTimeout time.Duration, wantSkipInsecure bool) {
	t.Helper()

	require.NotNil(t, client)
	assert.Equal(t, wantTimeout, client.Timeout)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.Proxy)

	if wantSkipInsecure {
		require.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
		return
	}
	assert.False(t, transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify)
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
