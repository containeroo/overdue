package config

import (
	"net/http"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/containeroo/tinyflags"
	"github.com/stretchr/testify/require"
)

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
	webhookGroup.Bool("skip-insecure", false, "Skip TLS certificate verification")
	webhookGroup.Bool("send-resolved", false, "Send resolved notifications")
	webhookGroup.String("title-template", `[OVERDUE] Event Notification`, "Title template")
	webhookGroup.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Resolved title template")
	webhookGroup.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Text template")
	webhookGroup.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Resolved text template")
	webhookGroup.StringSlice("headers", nil, "HTTP headers")
	webhookGroup.StringSlice("custom-data", nil, "Custom data")
	webhookGroup.String("template", "", "Body template")
	tinyflags.DynamicEnum(
		webhookGroup,
		"log-response",
		webhook.LogResponseSummary,
		"Webhook response logging",
		webhook.LogResponseSummary,
		webhook.LogResponseBody,
		webhook.LogResponseFull,
		webhook.LogResponseNone,
	)
	webhookGroup.Int("response-body-limit", 4096, "Response body limit")

	emailGroup := fs.DynamicGroup("email")
	emailGroup.String("smtp-host", "", "SMTP host")
	emailGroup.Int("smtp-port", 587, "SMTP port")
	emailGroup.String("smtp-user", "", "SMTP username")
	emailGroup.String("smtp-pass", "", "SMTP password")
	emailGroup.Bool("smtp-skip-insecure", false, "Skip SMTP TLS certificate verification")
	emailGroup.Bool("send-resolved", false, "Send resolved notifications")
	emailGroup.String("subject-template", "{{ .Title }}", "Subject template")
	emailGroup.String("resolved-subject-template", "{{ .Title }}", "Resolved subject template")
	emailGroup.String("title-template", `[OVERDUE] Event Notification`, "Title template")
	emailGroup.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Resolved title template")
	emailGroup.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Text template")
	emailGroup.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Resolved text template")
	emailGroup.String("from", "", "Sender")
	emailGroup.StringSlice("to", []string{}, "Recipients")
	emailGroup.StringSlice("headers", nil, "Email headers")
	emailGroup.StringSlice("custom-data", nil, "Custom data")
	emailGroup.String("template", "", "Body template")

	require.NoError(t, fs.Parse(args))
	return fs
}
