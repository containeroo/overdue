package flag

import (
	"net/http"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookConfigsFromDynamicGroup(t *testing.T) {
	t.Parallel()

	t.Run("converts configured webhook", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.headers=Authorization=Bearer token",
			"--webhook.ops.custom-data=channel=#ops",
			"--webhook.ops.method=PATCH",
			"--webhook.ops.timeout=5s",
			"--webhook.ops.skip-insecure=true",
			"--webhook.ops.send-resolved=true",
			"--webhook.ops.title-template=ops title",
			"--webhook.ops.resolved-title-template=ops resolved title",
			"--webhook.ops.text-template=ops text",
			"--webhook.ops.resolved-text-template=ops resolved text",
			"--webhook.ops.template=ops.tmpl",
			"--webhook.ops.log-response=body",
			"--webhook.ops.response-body-limit=128",
		})

		configs, err := webhookConfigsFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, targets.WebhookConfig{
			Name:         "ops",
			URL:          "https://example.test/webhook",
			Method:       http.MethodPatch,
			Headers:      map[string]string{"Authorization": "Bearer token", "User-Agent": "overdue/dev"},
			Timeout:      5 * time.Second,
			SkipInsecure: true,
			SendResolved: true,
			ContentTemplates: render.ContentTemplates{
				Title:         "ops title",
				ResolvedTitle: "ops resolved title",
				Text:          "ops text",
				ResolvedText:  "ops resolved text",
				CustomData:    map[string]string{"channel": "#ops"},
			},
			Template:          "ops.tmpl",
			LogResponse:       targets.LogResponseBody,
			ResponseBodyLimit: 128,
		}, configs[0])
	})
}

func TestWebhookConfigsFromDynamicGroupDefaults(t *testing.T) {
	t.Parallel()

	fs := notifyTestFlagSet(t, []string{"--webhook.ops.url=https://example.test/webhook"})
	webhookGroup := fs.DynamicGroup("webhook")

	configs, err := webhookConfigsFromDynamicGroup("dev", webhookGroup)

	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Equal(t, targets.WebhookConfig{
		Name:              "ops",
		Method:            http.MethodPost,
		URL:               "https://example.test/webhook",
		Headers:           map[string]string{"User-Agent": "overdue/dev"},
		Timeout:           10 * time.Second,
		ContentTemplates:  notifyTestDefaultContentTemplates(),
		LogResponse:       targets.LogResponseSummary,
		ResponseBodyLimit: 4096,
	}, configs[0])
}

func TestWebhookConfigsFromDynamicGroupErrors(t *testing.T) {
	t.Parallel()

	t.Run("returns header parse error", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.headers=invalid",
		})

		_, err := webhookConfigsFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.headers"`)
	})

	t.Run("returns custom data parse error", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.custom-data=invalid",
		})

		_, err := webhookConfigsFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.custom-data"`)
	})
}
