package notify

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/notifykit/targets/webhook"
	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReceiversFromConfig tests notifykit receiver construction.
func TestReceiversFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("keeps webhook and email receivers with the same name separate", func(t *testing.T) {
		t.Parallel()

		templatePaths := writeReceiverTemplates(t)

		receivers, router, err := ReceiversFromConfig(nil, config.Notifications{
			App: config.AppData{Version: "dev"},
			Webhooks: []config.WebhookConfig{
				{
					Name:              "ops",
					URL:               "https://example.test/webhook",
					Method:            http.MethodPost,
					Timeout:           time.Second,
					SendResolved:      true,
					SubjectTemplate:   `{{ if .Resolved }}[RESOLVED] Event Notification{{ else }}[OVERDUE] Event Notification{{ end }}`,
					Template:          templatePaths.webhook,
					ResponseBodyLimit: 128,
				},
			},
			Emails: []config.EmailConfig{
				{
					Name:            "ops",
					Host:            "smtp.example.test",
					Port:            587,
					SendResolved:    true,
					From:            "overdue@example.test",
					To:              []string{"ops@example.test"},
					SubjectTemplate: `{{ if .Resolved }}[RESOLVED] Event Notification{{ else }}[OVERDUE] Event Notification{{ end }}`,
					Template:        templatePaths.email,
				},
			},
		}, nil)

		require.NoError(t, err)
		assert.Contains(t, receivers, kit.ReceiverID("webhook.ops"))
		assert.Contains(t, receivers, kit.ReceiverID("email.ops"))
		assert.Len(t, receivers, 2)
		ids, ok := router.ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true})
		assert.True(t, ok)
		assert.Equal(t, []kit.ReceiverID{"webhook.ops", "email.ops"}, ids)
	})
}

// TestRouter_ReceiverIDsForEvent tests resolved event receiver routing.
func TestRouter_ReceiverIDsForEvent(t *testing.T) {
	t.Parallel()

	t.Run("returns all receivers for alerting event", func(t *testing.T) {
		t.Parallel()

		router := NewRouter([]kit.ReceiverID{"ops"})
		ids, ok := router.ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusAlerting})

		assert.True(t, ok)
		assert.Nil(t, ids)
	})

	t.Run("returns resolved receivers for resolved event", func(t *testing.T) {
		t.Parallel()

		router := NewRouter([]kit.ReceiverID{"ops", "email"})
		ids, ok := router.ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true})

		assert.True(t, ok)
		assert.Equal(t, []kit.ReceiverID{"ops", "email"}, ids)
	})

	t.Run("skips resolved event without resolved receivers", func(t *testing.T) {
		t.Parallel()

		router := NewRouter(nil)
		ids, ok := router.ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true})

		assert.False(t, ok)
		assert.Nil(t, ids)
	})

	t.Run("returns a defensive receiver copy", func(t *testing.T) {
		t.Parallel()

		original := []kit.ReceiverID{"ops"}
		router := NewRouter(original)
		original[0] = "changed"
		ids, ok := router.ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true})
		ids[0] = "changed"

		assert.True(t, ok)
		next, nextOK := router.ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true})
		assert.True(t, nextOK)
		assert.Equal(t, []kit.ReceiverID{"ops"}, next)
	})
}

// TestWebhookReceiverID tests webhook receiver ID construction.
func TestWebhookReceiverID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, kit.ReceiverID("webhook.ops"), webhookReceiverID("ops"))
}

// TestEmailReceiverID tests email receiver ID construction.
func TestEmailReceiverID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, kit.ReceiverID("email.ops"), emailReceiverID("ops"))
}

func TestWebhookLogResponse(t *testing.T) {
	t.Parallel()

	t.Run("converts known values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, webhook.LogResponseSummary, webhookLogResponse(config.WebhookLogResponseSummary))
		assert.Equal(t, webhook.LogResponseBody, webhookLogResponse(config.WebhookLogResponseBody))
		assert.Equal(t, webhook.LogResponseFull, webhookLogResponse(config.WebhookLogResponseFull))
		assert.Equal(t, webhook.LogResponseNone, webhookLogResponse(config.WebhookLogResponseNone))
	})

	t.Run("passes through empty values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, webhook.LogResponse(""), webhookLogResponse(""))
	})
}

type receiverTemplatePaths struct {
	webhook string
	email   string
}

// writeReceiverTemplates writes real filesystem template fixtures.
func writeReceiverTemplates(t *testing.T) receiverTemplatePaths {
	t.Helper()

	dir := t.TempDir()
	paths := receiverTemplatePaths{
		webhook: filepath.Join(dir, "webhook.tmpl"),
		email:   filepath.Join(dir, "email.tmpl"),
	}

	require.NoError(t, os.WriteFile(paths.webhook, []byte(`{"text":{{ json .Text }}}`), 0o600))
	require.NoError(t, os.WriteFile(paths.email, []byte(`{{ .Title }}`), 0o600))

	return paths
}
