package flag

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/utils"
	"github.com/containeroo/tinyflags"
)

// notificationsFromDynamicGroups converts parsed dynamic CLI groups into notification configuration.
func notificationsFromDynamicGroups(version, siteRoot, checkInPath string, groups []*tinyflags.DynamicGroup) (config.Notifications, error) {
	cfg := config.Notifications{
		App: config.NewAppData(version, siteRoot, checkInPath),
	}

	for _, group := range groups {
		switch group.Name() {
		case "webhook":
			webhooks, err := webhooksFromDynamicGroup(version, group)
			if err != nil {
				return config.Notifications{}, err
			}
			cfg.Webhooks = append(cfg.Webhooks, webhooks...)
		case "email":
			emails, err := emailsFromDynamicGroup(version, group)
			if err != nil {
				return config.Notifications{}, err
			}
			cfg.Emails = append(cfg.Emails, emails...)
		default:
			if len(group.Instances()) > 0 {
				return config.Notifications{}, fmt.Errorf("unsupported notification group %q", group.Name())
			}
		}
	}

	return cfg, nil
}

// webhooksFromDynamicGroup converts one parsed webhook group into typed config.
func webhooksFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]config.WebhookConfig, error) {
	ids := sortedInstances(group)
	configs := make([]config.WebhookConfig, 0, len(ids))

	for _, id := range ids {
		headers, err := config.HeaderMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
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

		customData, err := utils.KeyValueMap(group.Name(), id, "custom-data", tinyflags.GetOrDefaultDynamic[[]string](group, id, "custom-data"))
		if err != nil {
			return nil, err
		}

		configs = append(configs, config.WebhookConfig{
			Name:              id,
			URL:               tinyflags.GetOrDefaultDynamic[string](group, id, "url"),
			Method:            method,
			Headers:           headers,
			Timeout:           tinyflags.GetOrDefaultDynamic[time.Duration](group, id, "timeout"),
			TLSSkipVerify:     tinyflags.GetOrDefaultDynamic[bool](group, id, "tls-skip-verify"),
			SendResolved:      tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			SubjectTemplate:   tinyflags.GetOrDefaultDynamic[string](group, id, "subject-template"),
			Template:          tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
			CustomData:        customData,
			LogResponse:       tinyflags.GetOrDefaultDynamic[config.LogResponse](group, id, "log-response"),
			ResponseBodyLimit: tinyflags.GetOrDefaultDynamic[int](group, id, "response-body-limit"),
		})
	}

	return configs, nil
}

// emailsFromDynamicGroup converts one parsed email group into typed config.
func emailsFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]config.EmailConfig, error) {
	ids := sortedInstances(group)
	configs := make([]config.EmailConfig, 0, len(ids))

	for _, id := range ids {
		headers, err := config.HeaderMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = make(map[string]string, 1)
		}
		headers["X-Mailer"] = "overdue/" + version

		customData, err := utils.KeyValueMap(group.Name(), id, "custom-data", tinyflags.GetOrDefaultDynamic[[]string](group, id, "custom-data"))
		if err != nil {
			return nil, err
		}

		configs = append(configs, config.EmailConfig{
			Name:              id,
			Host:              tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-host"),
			Port:              tinyflags.GetOrDefaultDynamic[int](group, id, "smtp-port"),
			User:              tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-user"),
			Pass:              tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-pass"),
			SMTPTLSSkipVerify: tinyflags.GetOrDefaultDynamic[bool](group, id, "smtp-tls-skip-verify"),
			SendResolved:      tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			From:              tinyflags.GetOrDefaultDynamic[string](group, id, "from"),
			To:                tinyflags.GetOrDefaultDynamic[[]string](group, id, "to"),
			Headers:           headers,
			SubjectTemplate:   tinyflags.GetOrDefaultDynamic[string](group, id, "subject-template"),
			Template:          tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
			CustomData:        customData,
		})
	}

	return configs, nil
}

// sortedInstances returns dynamic group instance IDs in deterministic order.
func sortedInstances(group *tinyflags.DynamicGroup) (ids []string) {
	ids = append([]string(nil), group.Instances()...)
	sort.Strings(ids)
	return ids
}
