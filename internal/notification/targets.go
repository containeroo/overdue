package notification

import (
	"io/fs"
	"log/slog"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/target"
)

// targetFamily builds and validates one homogeneous notification target family.
type targetFamily interface {
	count() int
	build(templateFS fs.FS, app render.AppData, logger *slog.Logger) ([]target.Notifier, error)
	validate(templateFS fs.FS, app render.AppData, alertingEvent, resolvedEvent monitor.Event) error
}

func notificationFamilies(cfg config.Notifications) []targetFamily {
	return []targetFamily{
		webhookFamily{configs: cfg.Webhooks},
		emailFamily{configs: cfg.Emails},
	}
}

func buildTargets(templateFS fs.FS, cfg config.Notifications, logger *slog.Logger) ([]target.Notifier, error) {
	families := notificationFamilies(cfg)
	notifiers := make([]target.Notifier, 0, targetCount(families))

	for _, family := range families {
		built, err := family.build(templateFS, cfg.App, logger)
		if err != nil {
			return nil, err
		}
		notifiers = append(notifiers, built...)
	}

	return notifiers, nil
}

func validateTemplates(templateFS fs.FS, cfg config.Notifications, alertingEvent, resolvedEvent monitor.Event) error {
	for _, family := range notificationFamilies(cfg) {
		if err := family.validate(templateFS, cfg.App, alertingEvent, resolvedEvent); err != nil {
			return err
		}
	}
	return nil
}

func targetCount(families []targetFamily) int {
	count := 0
	for _, family := range families {
		count += family.count()
	}
	return count
}

func withAppData(content render.ContentTemplates, app render.AppData) render.ContentTemplates {
	content = content.Clone()
	content.App = app
	return content
}
