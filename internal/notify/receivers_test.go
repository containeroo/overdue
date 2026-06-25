package notify

import (
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	kit "github.com/containeroo/notifykit/notify"
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

		receivers, resolvedReceivers, err := ReceiversFromConfig(testTemplateFS(), config.Notifications{
			App: config.AppData{Version: "dev"},
			Webhooks: []config.WebhookConfig{
				{
					Name:              "ops",
					URL:               "https://example.test/webhook",
					Method:            http.MethodPost,
					Timeout:           time.Second,
					SendResolved:      true,
					SubjectTemplate:   config.DefaultSubjectTemplate(),
					Template:          "webhook.tmpl",
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
					SubjectTemplate: config.DefaultSubjectTemplate(),
					Template:        "email.tmpl",
				},
			},
		}, nil)

		require.NoError(t, err)
		assert.Contains(t, receivers, kit.ReceiverID("webhook.ops"))
		assert.Contains(t, receivers, kit.ReceiverID("email.ops"))
		assert.Len(t, receivers, 2)
		assert.Equal(t, []kit.ReceiverID{"webhook.ops", "email.ops"}, resolvedReceivers)
	})
}

// TestReceiverIDsForEvent tests resolved event receiver routing.
func TestReceiverIDsForEvent(t *testing.T) {
	t.Parallel()

	t.Run("returns all receivers for alerting event", func(t *testing.T) {
		t.Parallel()

		ids, ok := ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusAlerting}, []kit.ReceiverID{"ops"})

		assert.True(t, ok)
		assert.Nil(t, ids)
	})

	t.Run("returns resolved receivers for resolved event", func(t *testing.T) {
		t.Parallel()

		ids, ok := ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true}, []kit.ReceiverID{"ops", "email"})

		assert.True(t, ok)
		assert.Equal(t, []kit.ReceiverID{"ops", "email"}, ids)
	})

	t.Run("skips resolved event without resolved receivers", func(t *testing.T) {
		t.Parallel()

		ids, ok := ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true}, nil)

		assert.False(t, ok)
		assert.Nil(t, ids)
	})

	t.Run("returns a defensive receiver copy", func(t *testing.T) {
		t.Parallel()

		original := []kit.ReceiverID{"ops"}
		ids, ok := ReceiverIDsForEvent(monitor.Event{Status: monitor.StatusResolved, Resolved: true}, original)
		ids[0] = "changed"

		assert.True(t, ok)
		assert.Equal(t, []kit.ReceiverID{"ops"}, original)
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

// testTemplateFS returns notification template fixtures.
func testTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"webhook.tmpl": {
			Data: []byte(`{"text":{{ json .Text }}}`),
		},
		"email.tmpl": {
			Data: []byte(`{{ .Title }}`),
		},
	}
}
