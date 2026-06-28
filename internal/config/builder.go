package config

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/containeroo/overdue/internal/utils"

	"github.com/containeroo/tinyflags"
)

const (
	defaultSubjectTemplate = `{{ if .Resolved }}[RESOLVED]{{ else }}[OVERDUE]{{ end }} Event Notification`
)

// DefaultSubjectTemplate returns the default notification subject template.
func DefaultSubjectTemplate() string {
	return defaultSubjectTemplate
}

// FromFlags reads parsed dynamic notification groups into cfg.
func FromFlags(cfg *Config, version string, groups []*tinyflags.DynamicGroup) error {
	notifications, err := notificationsFromDynamicGroups(version, cfg.SiteRoot, cfg.CheckIn.Path, groups)
	if err != nil {
		return err
	}
	cfg.Notifications = notifications
	return nil
}

// notificationsFromDynamicGroups converts parsed dynamic CLI groups into notification configuration.
func notificationsFromDynamicGroups(version, siteRoot, checkInPath string, groups []*tinyflags.DynamicGroup) (Notifications, error) {
	cfg := Notifications{
		App: newAppData(version, siteRoot, checkInPath),
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

// newAppData builds template app data from normalized application settings.
func newAppData(version, siteRoot, checkInPath string) AppData {
	app := AppData{Version: version}
	if strings.TrimSpace(siteRoot) == "" {
		return app
	}

	app.SiteRoot = siteRoot
	app.CheckInURL = joinURLPath(siteRoot, checkInPath)
	app.StatusURL = joinURLPath(siteRoot, "/status")
	return app
}

// webhooksFromDynamicGroup converts one parsed webhook group into typed config.
func webhooksFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]WebhookConfig, error) {
	ids := sortedInstances(group)
	configs := make([]WebhookConfig, 0, len(ids))

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

		customData, err := utils.KeyValueMap(group.Name(), id, "custom-data", tinyflags.GetOrDefaultDynamic[[]string](group, id, "custom-data"))
		if err != nil {
			return nil, err
		}

		configs = append(configs, WebhookConfig{
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
			LogResponse:       tinyflags.GetOrDefaultDynamic[LogResponse](group, id, "log-response"),
			ResponseBodyLimit: tinyflags.GetOrDefaultDynamic[int](group, id, "response-body-limit"),
		})
	}

	return configs, nil
}

// emailsFromDynamicGroup converts one parsed email group into typed config.
func emailsFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]EmailConfig, error) {
	ids := sortedInstances(group)
	configs := make([]EmailConfig, 0, len(ids))

	for _, id := range ids {
		headers, err := headerMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
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

		configs = append(configs, EmailConfig{
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

// joinURLPath appends path to base using the simple URL behavior Overdue needs.
func joinURLPath(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
