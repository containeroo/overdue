package flag

import (
	"net/http"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/tinyflags"
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

		cfg, err := notificationsFromDynamicGroups("v0.0.7", "https://overdue.example.test/overdue", "/checkin", fs.DynamicGroups())

		require.NoError(t, err)
		assert.Equal(t, "v0.0.7", cfg.App.Version)
		assert.Equal(t, "https://overdue.example.test/overdue", cfg.App.SiteRoot)
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

		cfg, err := notificationsFromDynamicGroups("dev", "", "/checkin", fs.DynamicGroups())

		require.NoError(t, err)
		assert.Empty(t, cfg.Webhooks)
		assert.Empty(t, cfg.Emails)
	})

	t.Run("rejects unsupported populated dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")
		require.NoError(t, fs.Parse([]string{"--pagerduty.ops.routing-key=secret"}))

		_, err := notificationsFromDynamicGroups("dev", "", "/checkin", fs.DynamicGroups())

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
			"--webhook.ops.tls-skip-verify=true",
			"--webhook.ops.send-resolved=true",
			"--webhook.ops.subject-template=ops subject",
			"--webhook.ops.template=ops.tmpl",
			"--webhook.ops.log-response=body",
			"--webhook.ops.response-body-limit=128",
		})

		configs, err := webhooksFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, config.WebhookConfig{
			Name:              "ops",
			URL:               "https://example.test/webhook",
			Method:            http.MethodPatch,
			Headers:           map[string]string{"Authorization": "Bearer token", "User-Agent": "overdue/dev"},
			Timeout:           5 * time.Second,
			TLSSkipVerify:     true,
			SendResolved:      true,
			SubjectTemplate:   "ops subject",
			Template:          "ops.tmpl",
			CustomData:        map[string]string{"channel": "#ops"},
			LogResponse:       config.WebhookLogResponseBody,
			ResponseBodyLimit: 128,
		}, configs[0])
	})

	t.Run("uses defaults", func(t *testing.T) {
		t.Parallel()

		fs := notificationTestFlagSet(t, []string{"--webhook.ops.url=https://example.test/webhook"})

		configs, err := webhooksFromDynamicGroup("dev", fs.DynamicGroup("webhook"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, config.WebhookConfig{
			Name:              "ops",
			Method:            http.MethodPost,
			URL:               "https://example.test/webhook",
			Headers:           map[string]string{"User-Agent": "overdue/dev"},
			Timeout:           10 * time.Second,
			SubjectTemplate:   "",
			LogResponse:       config.WebhookLogResponseSummary,
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
			"--email.ops.smtp-tls-skip-verify=true",
			"--email.ops.send-resolved=true",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.headers=X-Trace=yes",
			"--email.ops.custom-data=owner=platform",
			"--email.ops.subject-template=subject",
			"--email.ops.template=email.tmpl",
		})

		configs, err := emailsFromDynamicGroup("dev", fs.DynamicGroup("email"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, config.EmailConfig{
			Name:              "ops",
			Host:              "smtp.example.test",
			Port:              2525,
			User:              "user",
			Pass:              "pass",
			SMTPTLSSkipVerify: true,
			SendResolved:      true,
			From:              "overdue@example.test",
			To:                []string{"ops@example.test"},
			Headers:           map[string]string{"X-Mailer": "overdue/dev", "X-Trace": "yes"},
			SubjectTemplate:   "subject",
			Template:          "email.tmpl",
			CustomData:        map[string]string{"owner": "platform"},
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
		assert.Equal(t, config.EmailConfig{
			Name:            "ops",
			Host:            "smtp.example.test",
			Port:            587,
			From:            "overdue@example.test",
			To:              []string{"ops@example.test"},
			Headers:         map[string]string{"X-Mailer": "overdue/dev"},
			SubjectTemplate: "",
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

// notificationTestFlagSet returns a parsed flag set for notification config tests.
func notificationTestFlagSet(t *testing.T, args []string) *tinyflags.FlagSet {
	t.Helper()

	fs := tinyflags.NewFlagSet("overdue", tinyflags.ContinueOnError)

	webhookGroup := fs.DynamicGroup("webhook")
	webhookGroup.String("url", "", "Webhook URL")
	tinyflags.DynamicEnum(
		webhookGroup,
		"method",
		http.MethodPost,
		"HTTP method",
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	)
	webhookGroup.Duration("timeout", 10*time.Second, "HTTP timeout")
	webhookGroup.Bool("tls-skip-verify", false, "Skip TLS certificate verification")
	webhookGroup.Bool("send-resolved", false, "Send resolved notifications")
	webhookGroup.String("subject-template", "", "Subject template")
	webhookGroup.StringSlice("headers", nil, "HTTP headers")
	webhookGroup.StringSlice("custom-data", nil, "Custom data")
	webhookGroup.String("template", "", "Body template")
	tinyflags.DynamicEnum(
		webhookGroup,
		"log-response",
		config.WebhookLogResponseSummary,
		"Webhook response logging",
		config.WebhookLogResponseSummary,
		config.WebhookLogResponseBody,
		config.WebhookLogResponseFull,
		config.WebhookLogResponseNone,
	)
	webhookGroup.Int("response-body-limit", 4096, "Response body limit")

	emailGroup := fs.DynamicGroup("email")
	emailGroup.String("smtp-host", "", "SMTP host")
	emailGroup.Int("smtp-port", 587, "SMTP port")
	emailGroup.String("smtp-user", "", "SMTP username")
	emailGroup.String("smtp-pass", "", "SMTP password")
	emailGroup.Bool("smtp-tls-skip-verify", false, "Skip SMTP TLS certificate verification")
	emailGroup.Bool("smtp-skip-insecure", false, "Deprecated alias for smtp-tls-skip-verify")
	emailGroup.Bool("send-resolved", false, "Send resolved notifications")
	emailGroup.String("subject-template", "", "Subject template")
	emailGroup.String("from", "", "Sender")
	emailGroup.StringSlice("to", []string{}, "Recipients")
	emailGroup.StringSlice("headers", nil, "Email headers")
	emailGroup.StringSlice("custom-data", nil, "Custom data")
	emailGroup.String("template", "", "Body template")

	require.NoError(t, fs.Parse(args))
	return fs
}
