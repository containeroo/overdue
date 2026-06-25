package notification

import (
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/target"
)

type emailFamily struct {
	configs []email.Config
}

func (f emailFamily) count() int {
	return len(f.configs)
}

func (f emailFamily) build(templateFS fs.FS, app render.AppData, logger *slog.Logger) ([]target.Notifier, error) {
	out := make([]target.Notifier, 0, len(f.configs))
	for _, cfg := range f.configs {
		cfg.ContentTemplates = withAppData(cfg.ContentTemplates, app)

		renderer, err := email.NewRenderer(
			templateFS,
			cfg.Template,
			cfg.SubjectTemplate,
			cfg.ResolvedSubjectTemplate,
			cfg.ContentTemplates,
		)
		if err != nil {
			return nil, fmt.Errorf("build email %q renderer: %w", cfg.Name, err)
		}

		out = append(out, email.New(
			cfg,
			renderer,
			logger.With("targetType", "email", "target", cfg.Name),
		))
	}
	return out, nil
}

func (f emailFamily) validate(templateFS fs.FS, app render.AppData, alertingEvent, resolvedEvent monitor.Event) error {
	for _, cfg := range f.configs {
		cfg.ContentTemplates = withAppData(cfg.ContentTemplates, app)

		renderer, err := email.NewRenderer(
			templateFS,
			cfg.Template,
			cfg.SubjectTemplate,
			cfg.ResolvedSubjectTemplate,
			cfg.ContentTemplates,
		)
		if err != nil {
			return fmt.Errorf("build email %q renderer: %w", cfg.Name, err)
		}
		if err := renderer.ValidateWithEvents(alertingEvent, resolvedEvent); err != nil {
			return fmt.Errorf("validate email %q templates: %w", cfg.Name, err)
		}
	}
	return nil
}
