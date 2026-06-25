package notification

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

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
			Data: []byte(`{"attachments":[],"channel":{{ json (.CustomData.channel | default "#alertmanager") }},"text":{{ json .Text }}}`),
		},
		"slack-chat-post-message.tmpl": {
			Data: []byte(`{"channel":{{ json (.CustomData.channel | default "#alertmanager") }},"text":{{ json .Text }}}`),
		},
	}
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
