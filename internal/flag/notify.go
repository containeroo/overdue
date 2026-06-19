package flag

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/containeroo/httputils"
	"github.com/containeroo/overdue/internal/notification/notifier"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/containeroo/tinyflags"
)

// notifyConfigFromDynamicGroups converts parsed dynamic CLI groups into typed notification config.
func notifyConfigFromDynamicGroups(version string, groups []*tinyflags.DynamicGroup) (notifier.Config, error) {
	cfg := notifier.Config{}

	for _, group := range groups {
		switch group.Name() {
		case "webhook":
			webhooks, err := webhookConfigsFromDynamicGroup(version, group)
			if err != nil {
				return notifier.Config{}, err
			}
			cfg.Webhooks = append(cfg.Webhooks, webhooks...)
		case "email":
			emails, err := emailConfigsFromDynamicGroup(version, group)
			if err != nil {
				return notifier.Config{}, err
			}
			cfg.Emails = append(cfg.Emails, emails...)
		default:
			if len(group.Instances()) > 0 {
				return notifier.Config{}, fmt.Errorf("unsupported notification group %q", group.Name())
			}
		}
	}

	return cfg, nil
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

		configs = append(configs, targets.WebhookConfig{
			Name:              id,
			URL:               tinyflags.GetOrDefaultDynamic[string](group, id, "url"),
			Headers:           headers,
			Timeout:           tinyflags.GetOrDefaultDynamic[time.Duration](group, id, "timeout"),
			SkipInsecure:      tinyflags.GetOrDefaultDynamic[bool](group, id, "skip-insecure"),
			SendResolved:      tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			ContentTemplates:  contentTemplates(group, id),
			Template:          tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
			LogResponse:       tinyflags.GetOrDefaultDynamic[targets.LogResponse](group, id, "log-response"),
			ResponseBodyLimit: tinyflags.GetOrDefaultDynamic[int](group, id, "response-body-limit"),
		})
	}

	return configs, nil
}

// emailConfigsFromDynamicGroup converts one parsed email dynamic group into typed config.
func emailConfigsFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]targets.EmailConfig, error) {
	ids := sortedInstances(group)
	configs := make([]targets.EmailConfig, 0, len(ids))

	for _, id := range ids {
		headers, err := webhookHeadersMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = make(map[string]string, 1)
		}
		headers["X-Mailer"] = "overdue/" + version

		configs = append(configs, targets.EmailConfig{
			Name:                    id,
			Host:                    tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-host"),
			Port:                    tinyflags.GetOrDefaultDynamic[int](group, id, "smtp-port"),
			User:                    tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-user"),
			Pass:                    tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-pass"),
			SkipTLSVerify:           tinyflags.GetOrDefaultDynamic[bool](group, id, "smtp-skip-insecure"),
			SendResolved:            tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			From:                    tinyflags.GetOrDefaultDynamic[string](group, id, "from"),
			To:                      tinyflags.GetOrDefaultDynamic[[]string](group, id, "to"),
			Headers:                 headers,
			SubjectTemplate:         tinyflags.GetOrDefaultDynamic[string](group, id, "subject-template"),
			ResolvedSubjectTemplate: tinyflags.GetOrDefaultDynamic[string](group, id, "resolved-subject-template"),
			ContentTemplates:        contentTemplates(group, id),
			Template:                tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
		})
	}

	return configs, nil
}

// contentTemplates reads title and text templates from a dynamic group instance.
func contentTemplates(group *tinyflags.DynamicGroup, id string) render.ContentTemplates {
	return render.ContentTemplates{
		Title:         tinyflags.GetOrDefaultDynamic[string](group, id, "title-template"),
		ResolvedTitle: tinyflags.GetOrDefaultDynamic[string](group, id, "resolved-title-template"),
		Text:          tinyflags.GetOrDefaultDynamic[string](group, id, "text-template"),
		ResolvedText:  tinyflags.GetOrDefaultDynamic[string](group, id, "resolved-text-template"),
	}
}

// webhookHeadersMap creates a header map from KEY=VALUE strings.
func webhookHeadersMap(groupName, id string, headers []string) (parsed map[string]string, err error) {
	if len(headers) == 0 {
		return nil, nil
	}

	parsed = make(map[string]string, len(headers))
	for _, header := range headers {
		values, err := httputils.ParseHeaders(header, false)
		if err != nil {
			return nil, fmt.Errorf("invalid \"--%s.%s.headers\": %w", groupName, id, err)
		}

		for name, value := range values {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("invalid \"--%s.%s.headers\": header name must not be empty", groupName, id)
			}
			parsed[name] = value
		}
	}

	return parsed, nil
}

// sortedInstances returns dynamic group instance IDs in deterministic order.
func sortedInstances(group *tinyflags.DynamicGroup) (ids []string) {
	ids = append([]string(nil), group.Instances()...)
	sort.Strings(ids)
	return ids
}
