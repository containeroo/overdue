package flag

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/containeroo/tinyflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyConfigFromDynamicGroups(t *testing.T) {
	t.Parallel()

	t.Run("converts webhooks and emails in sorted instance order", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--webhook.zeta.url=https://example.test/zeta",
			"--webhook.zeta.headers=Authorization=Bearer token",
			"--webhook.zeta.timeout=5s",
			"--webhook.zeta.skip-insecure=true",
			"--webhook.zeta.send-resolved=true",
			"--webhook.zeta.title-template=zeta title",
			"--webhook.zeta.resolved-title-template=zeta resolved title",
			"--webhook.zeta.text-template=zeta text",
			"--webhook.zeta.resolved-text-template=zeta resolved text",
			"--webhook.zeta.template=zeta.tmpl",
			"--webhook.zeta.log-response=body",
			"--webhook.zeta.response-body-limit=128",
			"--webhook.alpha.url=https://example.test/alpha",
			"--webhook.alpha.template=alpha.tmpl",
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.smtp-port=2525",
			"--email.ops.smtp-user=user",
			"--email.ops.smtp-pass=pass",
			"--email.ops.smtp-skip-insecure=true",
			"--email.ops.send-resolved=true",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.headers=X-Trace=yes",
			"--email.ops.subject-template=subject",
			"--email.ops.resolved-subject-template=resolved subject",
			"--email.ops.title-template=email title",
			"--email.ops.resolved-title-template=email resolved title",
			"--email.ops.text-template=email text",
			"--email.ops.resolved-text-template=email resolved text",
			"--email.ops.template=email.tmpl",
		})

		cfg, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.NoError(t, err)
		require.Len(t, cfg.Webhooks, 2)
		assert.Equal(t, "alpha", cfg.Webhooks[0].Name)
		assert.Equal(t, "zeta", cfg.Webhooks[1].Name)
		assert.Equal(t, targets.WebhookConfig{
			Name:              "zeta",
			URL:               "https://example.test/zeta",
			Headers:           map[string]string{"Authorization": "Bearer token", "User-Agent": "overdue/dev"},
			Timeout:           5 * time.Second,
			SkipInsecure:      true,
			SendResolved:      true,
			ContentTemplates:  notifyTestContentTemplates("zeta"),
			Template:          "zeta.tmpl",
			LogResponse:       targets.LogResponseBody,
			ResponseBodyLimit: 128,
		}, cfg.Webhooks[1])
		require.Len(t, cfg.Emails, 1)
		assert.Equal(t, targets.EmailConfig{
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
			},
			Template: "email.tmpl",
		}, cfg.Emails[0])
	})

	t.Run("ignores unsupported empty dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")

		cfg, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.NoError(t, err)
		assert.Empty(t, cfg.Webhooks)
		assert.Empty(t, cfg.Emails)
	})

	t.Run("rejects unsupported populated dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")
		require.NoError(t, fs.Parse([]string{"--pagerduty.ops.routing-key=secret"}))

		_, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.Error(t, err)
		assert.EqualError(t, err, `unsupported notification group "pagerduty"`)
	})

	t.Run("returns header parse error from webhook conversion", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.headers=invalid",
		})

		_, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.headers"`)
	})

	t.Run("returns header parse error from email conversion", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.headers=invalid",
		})

		_, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--email.ops.headers"`)
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
		URL:               "https://example.test/webhook",
		Headers:           map[string]string{"User-Agent": "overdue/dev"},
		Timeout:           10 * time.Second,
		ContentTemplates:  notifyTestDefaultContentTemplates(),
		LogResponse:       targets.LogResponseSummary,
		ResponseBodyLimit: 4096,
	}, configs[0])
}

func TestEmailConfigsFromDynamicGroupDefaults(t *testing.T) {
	t.Parallel()

	fs := notifyTestFlagSet(t, []string{
		"--email.ops.smtp-host=smtp.example.test",
		"--email.ops.from=overdue@example.test",
		"--email.ops.to=ops@example.test",
		"--email.ops.headers=X-Trace=yes",
		"--email.ops.headers=X-Mailer=custom",
	})
	emailGroup := fs.DynamicGroup("email")

	configs, err := emailConfigsFromDynamicGroup("dev", emailGroup)

	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Equal(t, targets.EmailConfig{
		Name:                    "ops",
		Host:                    "smtp.example.test",
		Port:                    587,
		From:                    "overdue@example.test",
		To:                      []string{"ops@example.test"},
		Headers:                 map[string]string{"X-Mailer": "overdue/dev", "X-Trace": "yes"},
		SubjectTemplate:         "{{ .Title }}",
		ResolvedSubjectTemplate: "{{ .Title }}",
		ContentTemplates:        notifyTestDefaultContentTemplates(),
	}, configs[0])
}

func TestSortedInstances(t *testing.T) {
	t.Parallel()

	fs := notifyTestFlagSet(t, []string{
		"--webhook.zeta.url=https://example.test/zeta",
		"--webhook.alpha.url=https://example.test/alpha",
		"--webhook.middle.url=https://example.test/middle",
	})

	assert.Equal(t, []string{"alpha", "middle", "zeta"}, sortedInstances(fs.DynamicGroup("webhook")))
}

func notifyTestFlagSet(t *testing.T, args []string) *tinyflags.FlagSet {
	t.Helper()
	fs := tinyflags.NewFlagSet("overdue", tinyflags.ContinueOnError)

	webhook := fs.DynamicGroup("webhook")
	webhook.String("url", "", "Webhook URL")
	webhook.Duration("timeout", 10*time.Second, "HTTP timeout")
	webhook.Bool("skip-insecure", false, "Skip TLS certificate verification")
	webhook.Bool("send-resolved", false, "Send resolved notifications")
	webhook.String("title-template", `[OVERDUE] Event Notification`, "Title template")
	webhook.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Resolved title template")
	webhook.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Text template")
	webhook.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Resolved text template")
	webhook.StringSlice("headers", nil, "HTTP headers")
	webhook.String("template", "", "Body template")
	tinyflags.DynamicEnum(
		webhook,
		"log-response",
		targets.LogResponseSummary,
		"Webhook response logging",
		targets.LogResponseSummary,
		targets.LogResponseBody,
		targets.LogResponseFull,
		targets.LogResponseNone,
	)
	webhook.Int("response-body-limit", 4096, "Response body limit")

	email := fs.DynamicGroup("email")
	email.String("smtp-host", "", "SMTP host")
	email.Int("smtp-port", 587, "SMTP port")
	email.String("smtp-user", "", "SMTP username")
	email.String("smtp-pass", "", "SMTP password")
	email.Bool("smtp-skip-insecure", false, "Skip SMTP TLS certificate verification")
	email.Bool("send-resolved", false, "Send resolved notifications")
	email.String("subject-template", "{{ .Title }}", "Subject template")
	email.String("resolved-subject-template", "{{ .Title }}", "Resolved subject template")
	email.String("title-template", `[OVERDUE] Event Notification`, "Title template")
	email.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Resolved title template")
	email.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Text template")
	email.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Resolved text template")
	email.String("from", "", "Sender")
	email.StringSlice("to", []string{}, "Recipients")
	email.StringSlice("headers", nil, "Email headers")
	email.String("template", "", "Body template")

	require.NoError(t, fs.Parse(args))
	return fs
}

func notifyTestContentTemplates(prefix string) render.ContentTemplates {
	return render.ContentTemplates{
		Title:         prefix + " title",
		ResolvedTitle: prefix + " resolved title",
		Text:          prefix + " text",
		ResolvedText:  prefix + " resolved text",
	}
}

func notifyTestDefaultContentTemplates() render.ContentTemplates {
	return render.ContentTemplates{
		Title:         `[OVERDUE] Event Notification`,
		ResolvedTitle: `[RESOLVED] [OVERDUE] Event Notification`,
		Text:          `Check-in "{{ .CheckInName }}" is overdue:`,
		ResolvedText:  `Check-in "{{ .CheckInName }}" is resolved:`,
	}
}
