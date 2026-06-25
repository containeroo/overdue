package config

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/containeroo/tinyflags"
)

// FromFlags reads parsed dynamic notification groups into cfg.
func FromFlags(cfg *Config, version string, groups []*tinyflags.DynamicGroup) error {
	notifications, err := NotificationsFromDynamicGroups(version, cfg.PublicURL, cfg.CheckIn.Path, groups)
	if err != nil {
		return err
	}
	cfg.Notifications = notifications
	return nil
}

// NotificationsFromDynamicGroups converts parsed dynamic CLI groups into notification configuration.
func NotificationsFromDynamicGroups(version, publicURL, checkInPath string, groups []*tinyflags.DynamicGroup) (Notifications, error) {
	cfg := Notifications{
		App: render.NewAppData(version, publicURL, checkInPath),
	}

	for _, group := range groups {
		switch group.Name() {
		case "webhook":
			webhooks, err := webhooksFromDynamicGroup(version, group)
			if err != nil {
				return Notifications{}, err
			}
			cfg.Webhooks = append(cfg.Webhooks, webhooks...)
		case "email":
			emails, err := emailsFromDynamicGroup(version, group)
			if err != nil {
				return Notifications{}, err
			}
			cfg.Emails = append(cfg.Emails, emails...)
		default:
			if len(group.Instances()) > 0 {
				return Notifications{}, fmt.Errorf("unsupported notification group %q", group.Name())
			}
		}
	}

	return cfg, nil
}

// webhooksFromDynamicGroup converts one parsed webhook group into typed config.
func webhooksFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]webhook.Config, error) {
	ids := sortedInstances(group)
	configs := make([]webhook.Config, 0, len(ids))

	for _, id := range ids {
		headers, err := headerMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
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

		configs = append(configs, webhook.Config{
			Name:              id,
			URL:               tinyflags.GetOrDefaultDynamic[string](group, id, "url"),
			Method:            method,
			Headers:           headers,
			Timeout:           tinyflags.GetOrDefaultDynamic[time.Duration](group, id, "timeout"),
			SkipInsecure:      tinyflags.GetOrDefaultDynamic[bool](group, id, "skip-insecure"),
			SendResolved:      tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			ContentTemplates:  content,
			Template:          tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
			LogResponse:       tinyflags.GetOrDefaultDynamic[webhook.LogResponse](group, id, "log-response"),
			ResponseBodyLimit: tinyflags.GetOrDefaultDynamic[int](group, id, "response-body-limit"),
		})
	}

	return configs, nil
}

// emailsFromDynamicGroup converts one parsed email group into typed config.
func emailsFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]email.Config, error) {
	ids := sortedInstances(group)
	configs := make([]email.Config, 0, len(ids))

	for _, id := range ids {
		headers, err := headerMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = make(map[string]string, 1)
		}
		headers["X-Mailer"] = "overdue/" + version

		customData, err := keyValueMap(group.Name(), id, "custom-data", tinyflags.GetOrDefaultDynamic[[]string](group, id, "custom-data"))
		if err != nil {
			return nil, err
		}

		content := contentTemplates(group, id)
		content.CustomData = customData

		configs = append(configs, email.Config{
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
			ContentTemplates:        content,
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

// sortedInstances returns dynamic group instance IDs in deterministic order.
func sortedInstances(group *tinyflags.DynamicGroup) (ids []string) {
	ids = append([]string(nil), group.Instances()...)
	sort.Strings(ids)
	return ids
}
