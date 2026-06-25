package notification

import (
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/containeroo/overdue/internal/notification/webhook"
)

type webhookFamily struct {
	configs []webhook.Config
}

func (f webhookFamily) count() int {
	return len(f.configs)
}

func (f webhookFamily) build(templateFS fs.FS, app render.AppData, logger *slog.Logger) ([]target.Notifier, error) {
	out := make([]target.Notifier, 0, len(f.configs))
	for _, cfg := range f.configs {
		cfg.ContentTemplates = withAppData(cfg.ContentTemplates, app)

		renderer, err := webhook.NewRenderer(templateFS, cfg.Template, cfg.ContentTemplates)
		if err != nil {
			return nil, fmt.Errorf("build webhook %q renderer: %w", cfg.Name, err)
		}

		out = append(out, webhook.New(
			cfg,
			renderer,
			logger.With("targetType", "webhook", "target", cfg.Name),
		))
	}
	return out, nil
}

func (f webhookFamily) validate(templateFS fs.FS, app render.AppData, alertingEvent, resolvedEvent monitor.Event) error {
	for _, cfg := range f.configs {
		cfg.ContentTemplates = withAppData(cfg.ContentTemplates, app)

		renderer, err := webhook.NewRenderer(templateFS, cfg.Template, cfg.ContentTemplates)
		if err != nil {
			return fmt.Errorf("build webhook %q renderer: %w", cfg.Name, err)
		}
		if err := renderer.ValidateWithEvents(alertingEvent, resolvedEvent); err != nil {
			return fmt.Errorf("validate webhook %q templates: %w", cfg.Name, err)
		}
	}
	return nil
}
