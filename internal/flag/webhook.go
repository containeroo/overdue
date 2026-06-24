package flag

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containeroo/httputils"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/containeroo/tinyflags"
)

// registerWebhookFlags registers dynamic webhook notification flags.
func registerWebhookFlags(tf *tinyflags.FlagSet) {
	webhook := tf.DynamicGroup("webhook").Title("Webhooks")
	webhook.String("url", "", "Webhook URL").
		Validate(func(raw string) error {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return nil
			}

			u, err := url.Parse(raw)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return errors.New("must be a valid URL")
			}
			return nil
		}).
		Required().
		Placeholder("URL")
	tinyflags.DynamicEnum(
		webhook,
		"method",
		http.MethodPost,
		"HTTP method used for webhook requests",
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	).Placeholder("METHOD")
	webhook.Duration("timeout", 10*time.Second, "HTTP timeout").
		Validate(func(value time.Duration) error {
			if value <= 0 {
				return errors.New("must be greater than zero")
			}
			return nil
		}).
		Placeholder("DURATION")
	webhook.Bool("skip-insecure", false, "Skip TLS certificate verification")
	webhook.Bool("send-resolved", false, "Send a resolved webhook notification when check-ins resume after alerting")
	webhook.String("title-template", `[OVERDUE] Event Notification`, "Template for overdue webhook title")
	webhook.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Template for resolved webhook title")
	webhook.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Template for overdue webhook text")
	webhook.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Template for resolved webhook text")
	webhook.StringSlice("headers", nil, "HTTP headers in KEY=VALUE format").
		Validate(func(raw string) error {
			_, err := httputils.ParseHeaders(raw, false)
			return err
		}).
		Placeholder("KEY=VALUE")
	webhook.StringSlice("custom-data", nil, "Custom webhook template data in KEY=VALUE format").
		Validate(validateKeyValue).
		Placeholder("KEY=VALUE")
	webhook.String("template", "", "Path or builtin:<name> template for webhook JSON body").
		Required().
		Placeholder("PATH|builtin:NAME")
	tinyflags.DynamicEnum(
		webhook,
		"log-response",
		targets.LogResponseSummary,
		"Webhook response logging: summary, body, full, or none",
		targets.LogResponseSummary,
		targets.LogResponseBody,
		targets.LogResponseFull,
		targets.LogResponseNone,
	).Placeholder("MODE")
	webhook.Int("response-body-limit", 4096, "Maximum webhook response body bytes to read for logs and errors").
		Validate(func(limit int) error {
			if limit <= 0 {
				return errors.New("response-body-limit must be a positive integer")
			}
			return nil
		}).
		Placeholder("BYTES")
}

// webhookConfigsFromDynamicGroup converts one parsed webhook dynamic group into typed config.
func webhookConfigsFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]targets.WebhookConfig, error) {
	ids := sortedInstances(group)
	configs := make([]targets.WebhookConfig, 0, len(ids))

	for _, id := range ids {
		headers, err := webhookHeadersMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = make(map[string]string, 1)
		}
		headers["User-Agent"] = "overdue/" + version

		method := tinyflags.GetOrDefaultDynamic[string](group, id, "method")
		if method == "" {
			method = http.MethodPost
		}

		customData, err := keyValueMap(group.Name(), id, "custom-data", tinyflags.GetOrDefaultDynamic[[]string](group, id, "custom-data"))
		if err != nil {
			return nil, err
		}

		content := contentTemplates(group, id)
		content.CustomData = customData

		configs = append(configs, targets.WebhookConfig{
			Name:              id,
			URL:               tinyflags.GetOrDefaultDynamic[string](group, id, "url"),
			Method:            method,
			Headers:           headers,
			Timeout:           tinyflags.GetOrDefaultDynamic[time.Duration](group, id, "timeout"),
			SkipInsecure:      tinyflags.GetOrDefaultDynamic[bool](group, id, "skip-insecure"),
			SendResolved:      tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			ContentTemplates:  content,
			Template:          tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
			LogResponse:       tinyflags.GetOrDefaultDynamic[targets.LogResponse](group, id, "log-response"),
			ResponseBodyLimit: tinyflags.GetOrDefaultDynamic[int](group, id, "response-body-limit"),
		})
	}

	return configs, nil
}
