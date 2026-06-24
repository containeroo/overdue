package flag

import (
	"fmt"
	"sort"

	"github.com/containeroo/overdue/internal/notification/notifier"
	"github.com/containeroo/overdue/internal/notification/render"
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
