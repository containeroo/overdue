package cli

import (
	"net/http"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/utils"
	"github.com/containeroo/tinyflags"
)

// registerWebhookFlags registers dynamic webhook notification flags.
func registerWebhookFlags(tf *tinyflags.FlagSet) {
	webhookGroup := tf.DynamicGroup("webhook").Title("Webhooks")

	webhookGroup.String("url", "", "Webhook URL").
		Validate(validateWebhookURL).
		Required().
		Placeholder("URL").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast)

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

	webhookGroup.Bool("tls-skip-verify", false, "Skip TLS certificate verification")
	webhookGroup.Bool("send-resolved", false, "Send a resolved webhook notification when check-ins resume after alerting")

	webhookGroup.String("title-template", defaultTitleTemplate, "Optional template for notification title")

	webhookGroup.StringSlice("headers", nil, "HTTP headers in KEY=VALUE format").
		Validate(validateHeader).
		Placeholder("KEY=VALUE").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast)

	webhookGroup.StringSlice("custom-data", nil, "Custom webhook template data in KEY=VALUE format").
		Validate(utils.ValidateKeyValue).
		Placeholder("KEY=VALUE")

	webhookGroup.String("template", "", "Path or builtin:<name> template for webhook JSON body").
		Required().
		Placeholder("PATH|builtin:NAME")

	tinyflags.DynamicEnum(
		webhookGroup,
		"log-response",
		config.WebhookLogResponseSummary,
		"Webhook response logging: summary, body, full, or none",
		config.WebhookLogResponseSummary,
		config.WebhookLogResponseBody,
		config.WebhookLogResponseFull,
		config.WebhookLogResponseNone,
	).Placeholder("MODE")

	webhookGroup.Int("response-body-limit", 4096, "Maximum webhook response body bytes to read for logs and errors").
		Validate(intAtLeast(1)).
		Placeholder("BYTES")
}
