package flag

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/containeroo/tinyflags"
	"github.com/stretchr/testify/require"
)

// notifyTestFlagSet returns a parsed flag set for notification config conversion tests.
func notifyTestFlagSet(t *testing.T, args []string) *tinyflags.FlagSet {
	t.Helper()

	fs := tinyflags.NewFlagSet("overdue", tinyflags.ContinueOnError)

	webhook := fs.DynamicGroup("webhook")
	webhook.String("url", "", "Webhook URL")
	tinyflags.DynamicEnum(
		webhook,
		"method",
		http.MethodPost,
		"HTTP method",
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	)
	webhook.Duration("timeout", 10*time.Second, "HTTP timeout")
	webhook.Bool("skip-insecure", false, "Skip TLS certificate verification")
	webhook.Bool("send-resolved", false, "Send resolved notifications")
	webhook.String("title-template", `[OVERDUE] Event Notification`, "Title template")
	webhook.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Resolved title template")
	webhook.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Text template")
	webhook.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Resolved text template")
	webhook.StringSlice("headers", nil, "HTTP headers")
	webhook.StringSlice("custom-data", nil, "Custom data")
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
	email.StringSlice("custom-data", nil, "Custom data")
	email.String("template", "", "Body template")

	require.NoError(t, fs.Parse(args))
	return fs
}

// notifyTestDefaultContentTemplates returns the default content template config.
func notifyTestDefaultContentTemplates() render.ContentTemplates {
	return render.ContentTemplates{
		Title:         `[OVERDUE] Event Notification`,
		ResolvedTitle: `[RESOLVED] [OVERDUE] Event Notification`,
		Text:          `Check-in "{{ .CheckInName }}" is overdue:`,
		ResolvedText:  `Check-in "{{ .CheckInName }}" is resolved:`,
	}
}

// writeTemplate writes a temporary template file.
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
