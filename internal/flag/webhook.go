package flag

import (
	"net/http"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/containeroo/tinyflags"
)

// registerWebhookFlags registers dynamic webhook notification flags.
func registerWebhookFlags(tf *tinyflags.FlagSet) {
	webhookGroup := tf.DynamicGroup("webhook").Title("Webhooks")

	webhookGroup.String("url", "", "Webhook URL").
		Validate(validateWebhookURL).
		Required().
		Placeholder("URL")

	tinyflags.DynamicEnum(
		webhookGroup,
		"method",
		http.MethodPost,
		"HTTP method used for webhook requests",
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	).Placeholder("METHOD")

	webhookGroup.Duration("timeout", 10*time.Second, "HTTP timeout").
		Validate(durationGreaterThanZero()).
		Placeholder("DURATION")

	webhookGroup.Bool("skip-insecure", false, "Skip TLS certificate verification")
	webhookGroup.Bool("send-resolved", false, "Send a resolved webhook notification when check-ins resume after alerting")

	webhookGroup.String("title-template", `[OVERDUE] Event Notification`, "Template for overdue webhook title")
	webhookGroup.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Template for resolved webhook title")

	webhookGroup.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Template for overdue webhook text")
	webhookGroup.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Template for resolved webhook text")

	webhookGroup.StringSlice("headers", nil, "HTTP headers in KEY=VALUE format").
		Validate(validateHeader).
		Placeholder("KEY=VALUE")

	webhookGroup.StringSlice("custom-data", nil, "Custom webhook template data in KEY=VALUE format").
		Validate(config.ValidateKeyValue).
		Placeholder("KEY=VALUE")

	webhookGroup.String("template", "", "Path or builtin:<name> template for webhook JSON body").
		Required().
		Placeholder("PATH|builtin:NAME")

	tinyflags.DynamicEnum(
		webhookGroup,
		"log-response",
		webhook.LogResponseSummary,
		"Webhook response logging: summary, body, full, or none",
		webhook.LogResponseSummary,
		webhook.LogResponseBody,
		webhook.LogResponseFull,
		webhook.LogResponseNone,
	).Placeholder("MODE")

	webhookGroup.Int("response-body-limit", 4096, "Maximum webhook response body bytes to read for logs and errors").
		Validate(intAtLeast(1)).
		Placeholder("BYTES")
}
