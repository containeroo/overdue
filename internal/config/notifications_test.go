package config

import (
	"net/http"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationsFromDynamicGroups(t *testing.T) {
	t.Parallel()

	t.Run("converts targets and app data", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{
			"--webhook.zeta.url=https://example.test/zeta",
			"--webhook.zeta.template=zeta.tmpl",
			"--webhook.alpha.url=https://example.test/alpha",
			"--webhook.alpha.template=alpha.tmpl",
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.template=email.tmpl",
		})

		cfg, err := NotificationsFromDynamicGroups("v0.0.7", "https://overdue.example.test/overdue", "/checkin", fs.DynamicGroups())

		require.NoError(t, err)
		assert.Equal(t, "v0.0.7", cfg.App.Version)
		assert.Equal(t, "https://overdue.example.test/overdue/checkin", cfg.App.CheckInURL)
		assert.Equal(t, "https://overdue.example.test/overdue/status", cfg.App.StatusURL)
		require.Len(t, cfg.Webhooks, 2)
		assert.Equal(t, "alpha", cfg.Webhooks[0].Name)
		assert.Equal(t, "zeta", cfg.Webhooks[1].Name)
		require.Len(t, cfg.Emails, 1)
		assert.Equal(t, "ops", cfg.Emails[0].Name)
	})

	t.Run("ignores unsupported empty dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")

		cfg, err := NotificationsFromDynamicGroups("dev", "", "/checkin", fs.DynamicGroups())

		require.NoError(t, err)
		assert.Empty(t, cfg.Webhooks)
		assert.Empty(t, cfg.Emails)
	})

	t.Run("rejects unsupported populated dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")
		require.NoError(t, fs.Parse([]string{"--pagerduty.ops.routing-key=secret"}))

		_, err := NotificationsFromDynamicGroups("dev", "", "/checkin", fs.DynamicGroups())

		require.Error(t, err)
		assert.EqualError(t, err, `unsupported notification group "pagerduty"`)
	})
}

func TestWebhooksFromDynamicGroup(t *testing.T) {
	t.Parallel()

	t.Run("converts configured webhook", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{
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

		configs, err := webhooksFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, webhook.Config{
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
			LogResponse:       webhook.LogResponseBody,
			ResponseBodyLimit: 128,
		}, configs[0])
	})

	t.Run("uses defaults", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{"--webhook.ops.url=https://example.test/webhook"})

		configs, err := webhooksFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, webhook.Config{
			Name:              "ops",
			Method:            http.MethodPost,
			URL:               "https://example.test/webhook",
			Headers:           map[string]string{"User-Agent": "overdue/dev"},
			Timeout:           10 * time.Second,
			ContentTemplates:  render.DefaultContentTemplates(),
			LogResponse:       webhook.LogResponseSummary,
			ResponseBodyLimit: 4096,
		}, configs[0])
	})

	t.Run("returns parse errors", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.custom-data=invalid",
		})

		_, err := webhooksFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.custom-data"`)
	})
}

func TestEmailsFromDynamicGroup(t *testing.T) {
	t.Parallel()

	t.Run("converts configured email", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.smtp-port=2525",
			"--email.ops.smtp-user=user",
			"--email.ops.smtp-pass=pass",
			"--email.ops.smtp-skip-insecure=true",
			"--email.ops.send-resolved=true",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.headers=X-Trace=yes",
			"--email.ops.custom-data=owner=platform",
			"--email.ops.subject-template=subject",
			"--email.ops.resolved-subject-template=resolved subject",
			"--email.ops.title-template=email title",
			"--email.ops.resolved-title-template=email resolved title",
			"--email.ops.text-template=email text",
			"--email.ops.resolved-text-template=email resolved text",
			"--email.ops.template=email.tmpl",
		})

		configs, err := emailsFromDynamicGroup("dev", fs.DynamicGroup("email"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, email.Config{
			Name:                    "ops",
			Host:                    "smtp.example.test",
			Port:                    2525,
			User:                    "user",
			Pass:                    "pass",
			SkipTLSVerify:           true,
			SendResolved:            true,
			From:                    "overdue@example.test",
			To:                      []string{"ops@example.test"},
			Headers:                 map[string]string{"X-Mailer": "overdue/dev", "X-Trace": "yes"},
			SubjectTemplate:         "subject",
			ResolvedSubjectTemplate: "resolved subject",
			ContentTemplates: render.ContentTemplates{
				Title:         "email title",
				ResolvedTitle: "email resolved title",
				Text:          "email text",
				ResolvedText:  "email resolved text",
				CustomData:    map[string]string{"owner": "platform"},
			},
			Template: "email.tmpl",
		}, configs[0])
	})

	t.Run("uses defaults", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
		})

		configs, err := emailsFromDynamicGroup("dev", fs.DynamicGroup("email"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, email.Config{
			Name:                    "ops",
			Host:                    "smtp.example.test",
			Port:                    587,
			From:                    "overdue@example.test",
			To:                      []string{"ops@example.test"},
			Headers:                 map[string]string{"X-Mailer": "overdue/dev"},
			SubjectTemplate:         "{{ .Title }}",
			ResolvedSubjectTemplate: "{{ .Title }}",
			ContentTemplates:        render.DefaultContentTemplates(),
		}, configs[0])
	})
}

func TestSortedInstances(t *testing.T) {
	t.Parallel()

	fs := notificationTestFlagSet(t, []string{
		"--webhook.zeta.url=https://example.test/zeta",
		"--webhook.alpha.url=https://example.test/alpha",
		"--webhook.middle.url=https://example.test/middle",
	})

	assert.Equal(t, []string{"alpha", "middle", "zeta"}, sortedInstances(fs.DynamicGroup("webhook")))
}
