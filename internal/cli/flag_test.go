package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	t.Run("normalizes path and prefix", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--route-prefix", "https://example.test/overdue/",
			"--name", "prometheus",
			"--path", "checkin/",
			"--expected-every", "10s",
			"--alerting-delay", "2s",
		}, "dev")

		require.NoError(t, err)
		assert.Equal(t, "/overdue", cfg.RoutePrefix)
		assert.Equal(t, "prometheus", cfg.CheckIn.Name)
		assert.Equal(t, "/checkin", cfg.CheckIn.Path)
		assert.Equal(t, 10*time.Second, cfg.CheckIn.ExpectedEvery)
		assert.Equal(t, 2*time.Second, cfg.CheckIn.AlertingDelay)
		assert.Equal(t, logging.LogFormatJSON, cfg.LogFormat)
	})

	t.Run("reads overdue env vars", func(t *testing.T) {
		t.Setenv("OVERDUE__NAME", "prometheus")
		t.Setenv("OVERDUE__PATH", "checkin")
		t.Setenv("OVERDUE__EXPECTED_EVERY", "10s")
		t.Setenv("OVERDUE__ALERTING_DELAY", "2s")
		t.Setenv("OVERDUE__LOG_FORMAT", "text")

		cfg, err := ParseArgs(nil, "dev")

		require.NoError(t, err)
		assert.Equal(t, "prometheus", cfg.CheckIn.Name)
		assert.Equal(t, "/checkin", cfg.CheckIn.Path)
		assert.Equal(t, 10*time.Second, cfg.CheckIn.ExpectedEvery)
		assert.Equal(t, 2*time.Second, cfg.CheckIn.AlertingDelay)
		assert.Equal(t, logging.LogFormatText, cfg.LogFormat)
	})

	t.Run("builds app template data from public url", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--public-url=https://overdue.example.test/overdue/",
			"--path=custom-check-in/",
		}, "v0.0.7")

		require.NoError(t, err)
		assert.Equal(t, "https://overdue.example.test/overdue", cfg.SiteRoot)
		assert.Equal(t, "v0.0.7", cfg.Notifications.App.Version)
		assert.Equal(t, "https://overdue.example.test/overdue", cfg.Notifications.App.SiteRoot)
		assert.Equal(t, "https://overdue.example.test/overdue/custom-check-in", cfg.Notifications.App.CheckInURL)
		assert.Equal(t, "https://overdue.example.test/overdue/status", cfg.Notifications.App.StatusURL)
	})

	t.Run("builds app template data with route prefix", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--public-url=https://overdue.example.test",
			"--route-prefix=/overdue",
		}, "v0.0.7")

		require.NoError(t, err)
		assert.Equal(t, "https://overdue.example.test/overdue", cfg.Notifications.App.SiteRoot)
		assert.Equal(t, "https://overdue.example.test/overdue/checkin", cfg.Notifications.App.CheckInURL)
		assert.Equal(t, "https://overdue.example.test/overdue/status", cfg.Notifications.App.StatusURL)
	})

	t.Run("rejects invalid public url", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--public-url=not-a-url",
		}, "dev")

		require.Error(t, err)
	})

	t.Run("reads startup and response detail flags", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--start-active",
			"--allow-get-checkin",
			"--response-details",
		}, "dev")

		require.NoError(t, err)
		assert.True(t, cfg.CheckIn.StartActive)
		assert.True(t, cfg.CheckIn.AllowGET)
		assert.True(t, cfg.ResponseDetails)
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
		require.Len(t, cfg.Notifications.Webhooks, 1)
		assert.Equal(t, "builtin:slack-incoming-webhook", cfg.Notifications.Webhooks[0].Template)
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
		require.Len(t, cfg.Notifications.Emails, 1)
		assert.Equal(t, "builtin:email-html", cfg.Notifications.Emails[0].Template)
	})

	t.Run("command line overrides dynamic env", func(t *testing.T) {
		templatePath := filepath.Join(t.TempDir(), "webhook.tmpl")
		require.NoError(t, os.WriteFile(templatePath, []byte(`{"text":{{ json .Text }}}`), 0o600))

		t.Setenv("OVERDUE__WEBHOOK_OPS_URL", "https://example.test/env")
		t.Setenv("OVERDUE__WEBHOOK_OPS_METHOD", "PUT")
		t.Setenv("OVERDUE__WEBHOOK_OPS_TIMEOUT", "3s")

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/cli",
			"--webhook.ops.timeout=7s",
			"--webhook.ops.template", templatePath,
		}, "dev")

		require.NoError(t, err)
		require.Len(t, cfg.Notifications.Webhooks, 1)
		assert.Equal(t, "https://example.test/cli", cfg.Notifications.Webhooks[0].URL)
		assert.Equal(t, 7*time.Second, cfg.Notifications.Webhooks[0].Timeout)
	})

	t.Run("records masked overridden values", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--auth-token=0123456789abcdefghijklmnopqrstuvwxyz",
			"--webhook.ops.url=https://hooks.example.test/secret",
			"--webhook.ops.headers=Authorization=Bearer webhook-secret",
			"--webhook.ops.template=builtin:slack-incoming-webhook",
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.smtp-user=user",
			"--email.ops.smtp-pass=email-secret",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.headers=X-Token=email-header-secret",
			"--email.ops.template=builtin:email-html",
		}, "dev")

		require.NoError(t, err)
		overrides := fmt.Sprint(cfg.OverriddenValues)
		assert.NotContains(t, overrides, "0123456789abcdefghijklmnopqrstuvwxyz")
		assert.NotContains(t, overrides, "https://hooks.example.test/secret")
		assert.NotContains(t, overrides, "Authorization=Bearer webhook-secret")
		assert.NotContains(t, overrides, "email-secret")
		assert.NotContains(t, overrides, "X-Token=email-header-secret")
		assert.Equal(t, "email-secret", cfg.Notifications.Emails[0].Pass)
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

	t.Run("rejects removed check-in name flag", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--check-in-name=prometheus", "--expected-every=10s", "--alerting-delay=2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects removed check-in path flag", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--check-in-path=/checkin", "--expected-every=10s", "--alerting-delay=2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects missing required durations", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs(nil, "dev")

		require.Error(t, err)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--name", " ", "--expected-every", "10s", "--alerting-delay", "2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects invalid expected duration", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--expected-every", "0s", "--alerting-delay", "2s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects invalid log format", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs([]string{"--expected-every=10s", "--alerting-delay=2s", "--log-format=pretty"}, "dev")

		require.Error(t, err)
	})

	t.Run("accepts renamed tls skip verify flags", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--expected-every=10s",
			"--alerting-delay=2s",
			"--webhook.ops.url=https://example.test/webhook",
			"--webhook.ops.template=builtin:slack-incoming-webhook",
			"--webhook.ops.tls-skip-verify",
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.template=builtin:email-html",
			"--email.ops.smtp-tls-skip-verify",
		}, "dev")

		require.NoError(t, err)
		require.Len(t, cfg.Notifications.Webhooks, 1)
		require.Len(t, cfg.Notifications.Emails, 1)
		assert.True(t, cfg.Notifications.Webhooks[0].TLSSkipVerify)
		assert.True(t, cfg.Notifications.Emails[0].SMTPTLSSkipVerify)
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
		require.Len(t, cfg.Notifications.Webhooks, 1)
		assert.Equal(t, path, cfg.Notifications.Webhooks[0].Template)
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
