package flag

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	t.Run("normalizes check-in path and prefix", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--route-prefix", "https://example.test/overdue/",
			"--check-in-name", "prometheus",
			"--check-in-path", "check-in/",
			"--expected-every", "10s",
			"--alerting-delay", "2s",
		}, "dev")

		require.NoError(t, err)
		assert.Equal(t, "/overdue", cfg.RoutePrefix)
		assert.Equal(t, "prometheus", cfg.CheckInName)
		assert.Equal(t, "/check-in", cfg.CheckInPath)
		assert.Equal(t, 10*time.Second, cfg.ExpectedEvery)
		assert.Equal(t, 2*time.Second, cfg.AlertingDelay)
		assert.Equal(t, logging.LogFormatJSON, cfg.LogFormat)
	})

	t.Run("reads overdue env vars", func(t *testing.T) {
		t.Setenv("OVERDUE__CHECK_IN_NAME", "prometheus")
		t.Setenv("OVERDUE__CHECK_IN_PATH", "check-in")
		t.Setenv("OVERDUE__EXPECTED_EVERY", "10s")
		t.Setenv("OVERDUE__ALERTING_DELAY", "2s")
		t.Setenv("OVERDUE__LOG_FORMAT", "text")

		cfg, err := ParseArgs(nil, "dev")

		require.NoError(t, err)
		assert.Equal(t, "prometheus", cfg.CheckInName)
		assert.Equal(t, "/check-in", cfg.CheckInPath)
		assert.Equal(t, 10*time.Second, cfg.ExpectedEvery)
		assert.Equal(t, 2*time.Second, cfg.AlertingDelay)
		assert.Equal(t, logging.LogFormatText, cfg.LogFormat)
	})

	t.Run("reads startup and response detail flags", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--start-active",
			"--response-details",
		}, "dev")

		require.NoError(t, err)
		assert.True(t, cfg.StartActive)
		assert.True(t, cfg.ResponseDetails)
	})

	t.Run("reads webhook env vars", func(t *testing.T) {
		templatePath := writeTemplate(t, `{"text":{{ json .Text }}}`)
		t.Setenv("OVERDUE__EXPECTED_EVERY", "10s")
		t.Setenv("OVERDUE__ALERTING_DELAY", "2s")
		t.Setenv("OVERDUE__WEBHOOK_OPS_URL", "https://example.test/webhook")
		t.Setenv("OVERDUE__WEBHOOK_OPS_TIMEOUT", "3s")
		t.Setenv("OVERDUE__WEBHOOK_OPS_SKIP_INSECURE", "true")
		t.Setenv("OVERDUE__WEBHOOK_OPS_SEND_RESOLVED", "true")
		t.Setenv("OVERDUE__WEBHOOK_OPS_LOG_RESPONSE", "body")
		t.Setenv("OVERDUE__WEBHOOK_OPS_TEMPLATE", templatePath)

		cfg, err := ParseArgs(nil, "dev")

		require.NoError(t, err)
		webhook := requireWebhookConfig(t, cfg.Notify.Webhooks, "ops")
		assert.Equal(t, "https://example.test/webhook", webhook.URL)
		assert.Equal(t, 3*time.Second, webhook.Timeout)
		assert.True(t, webhook.SkipInsecure)
		assert.True(t, webhook.SendResolved)
		assert.Equal(t, targets.LogResponseBody, webhook.LogResponse)
		assert.Equal(t, templatePath, webhook.Template)
	})

	t.Run("accepts builtin webhook template without checking embedded fs", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.template=builtin:slack-incoming-webhook",
		}, "dev")

		require.NoError(t, err)
		webhook := requireWebhookConfig(t, cfg.Notify.Webhooks, "ops")
		assert.Equal(t, "builtin:slack-incoming-webhook", webhook.Template)
	})

	t.Run("accepts unknown builtin webhook template because runtime validates templates", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.template=builtin:missing",
		}, "dev")

		require.NoError(t, err)
		webhook := requireWebhookConfig(t, cfg.Notify.Webhooks, "ops")
		assert.Equal(t, "builtin:missing", webhook.Template)
	})

	t.Run("rejects invalid log format", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--expected-every=10s", "--alerting-delay=2s", "--log-format=pretty"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects invalid webhook log response", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.log-response=everything",
		}, "dev")

		require.Error(t, err)
	})

	t.Run("reads email env vars", func(t *testing.T) {
		templatePath := writeTemplate(t, `body={{ .Title }}`)
		t.Setenv("OVERDUE__EXPECTED_EVERY", "10s")
		t.Setenv("OVERDUE__ALERTING_DELAY", "2s")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_SMTP_HOST", "smtp.example.test")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_SMTP_PORT", "2525")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_SMTP_USER", "user")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_SMTP_PASS", "pass")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_SMTP_SKIP_INSECURE", "true")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_SEND_RESOLVED", "true")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_FROM", "overdue@example.test")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_TO", "ops@example.test")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_HEADERS", "X-Trace=yes")
		t.Setenv("OVERDUE__EMAIL_PRIMARY_TEMPLATE", templatePath)

		cfg, err := ParseArgs(nil, "dev")

		require.NoError(t, err)
		email := requireEmailConfig(t, cfg.Notify.Emails, "primary")
		assert.Equal(t, "smtp.example.test", email.Host)
		assert.Equal(t, 2525, email.Port)
		assert.Equal(t, "user", email.User)
		assert.Equal(t, "pass", email.Pass)
		assert.True(t, email.SkipTLSVerify)
		assert.True(t, email.SendResolved)
		assert.Equal(t, "overdue@example.test", email.From)
		assert.Equal(t, []string{"ops@example.test"}, email.To)
		assert.Equal(t, map[string]string{"X-Mailer": "overdue/dev", "X-Trace": "yes"}, email.Headers)
	})

	t.Run("accepts builtin email template without checking embedded fs", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--email.primary.smtp-host=smtp.example.test",
			"--email.primary.smtp-user=user",
			"--email.primary.from=overdue@example.test",
			"--email.primary.to=ops@example.test",
			"--email.primary.template=builtin:email-html",
		}, "dev")

		require.NoError(t, err)
		email := requireEmailConfig(t, cfg.Notify.Emails, "primary")
		assert.Equal(t, "builtin:email-html", email.Template)
	})

	t.Run("command line overrides dynamic env", func(t *testing.T) {
		templatePath := writeTemplate(t, `{"text":{{ json .Text }}}`)
		t.Setenv("OVERDUE__WEBHOOK_OPS_URL", "https://example.test/env")
		t.Setenv("OVERDUE__WEBHOOK_OPS_TIMEOUT", "3s")

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/cli",
			"--webhook.ops.timeout=7s",
			"--webhook.ops.template", templatePath,
		}, "dev")

		require.NoError(t, err)
		webhook := requireWebhookConfig(t, cfg.Notify.Webhooks, "ops")
		assert.Equal(t, "https://example.test/cli", webhook.URL)
		assert.Equal(t, 7*time.Second, webhook.Timeout)
	})

	t.Run("rejects removed check-in history size flag", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--expected-every=10s", "--alerting-delay=2s", "--check-in-history-size=1"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects removed old delay flag", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--expected-every=10s", "--grace-period=2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects missing required durations", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs(nil, "dev")

		require.Error(t, err)
	})

	t.Run("rejects empty check-in name", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--check-in-name", " ", "--expected-every", "10s", "--alerting-delay", "2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects invalid expected duration", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--expected-every", "0s", "--alerting-delay", "2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects invalid webhook timeout", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.timeout=0s",
		}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects invalid webhook url", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url", "not-a-url",
		}, "dev")

		require.Error(t, err)
	})

	t.Run("accepts missing template path because runtime validates templates", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "missing.tmpl")
		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.template", path,
		}, "dev")

		require.NoError(t, err)
		webhook := requireWebhookConfig(t, cfg.Notify.Webhooks, "ops")
		assert.Equal(t, path, webhook.Template)
	})

	t.Run("rejects unsupported dynamic field", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.typo=value",
		}, "dev")

		require.Error(t, err)
	})
}

func TestWebhookHeadersMap(t *testing.T) {
	t.Parallel()
	t.Run("returns nil for empty headers", func(t *testing.T) {
		t.Parallel()

		headers, err := webhookHeadersMap("webhook", "ops", nil)

		require.NoError(t, err)
		assert.Nil(t, headers)
	})

	t.Run("parses headers", func(t *testing.T) {
		t.Parallel()

		headers, err := webhookHeadersMap("webhook", "ops", []string{"Authorization=Bearer token", "X-Test=yes"})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"Authorization": "Bearer token", "X-Test": "yes"}, headers)
	})

	t.Run("rejects invalid header", func(t *testing.T) {
		t.Parallel()

		_, err := webhookHeadersMap("webhook", "ops", []string{"invalid"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--webhook.ops.headers"`)
	})
}

// writeTemplate writes a temporary template.
func writeTemplate(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// requireWebhookConfig returns a named webhook config.
func requireWebhookConfig(t *testing.T, configs []targets.WebhookConfig, name string) targets.WebhookConfig {
	t.Helper()
	for _, cfg := range configs {
		if cfg.Name == name {
			return cfg
		}
	}
	require.FailNowf(t, "missing webhook config", "webhook %q not found", name)
	return targets.WebhookConfig{}
}

// requireEmailConfig returns a named email config.
func requireEmailConfig(t *testing.T, configs []targets.EmailConfig, name string) targets.EmailConfig {
	t.Helper()
	for _, cfg := range configs {
		if cfg.Name == name {
			return cfg
		}
	}
	require.FailNowf(t, "missing email config", "email %q not found", name)
	return targets.EmailConfig{}
}
