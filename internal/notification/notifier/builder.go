package notifier

import (
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/delivery"
	"github.com/containeroo/overdue/internal/notification/dispatch"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
)

// New creates the configured notifier backend.
func New(templateFS fs.FS, cfg Config, logger *slog.Logger) (delivery.Notifier, error) {
	if logger == nil {
		panic("notification setup logger must not be nil")
	}

	notifiers := make([]delivery.Notifier, 0, len(cfg.Webhooks)+len(cfg.Emails))

	webhooks, err := buildWebhookNotifiers(templateFS, cfg.App, cfg.Webhooks, logger)
	if err != nil {
		return nil, err
	}
	notifiers = append(notifiers, webhooks...)

	emails, err := buildEmailNotifiers(templateFS, cfg.App, cfg.Emails, logger)
	if err != nil {
		return nil, err
	}
	notifiers = append(notifiers, emails...)

	logger.Info(
		"configured notifiers",
		"total", len(notifiers),
		"webhooks", len(cfg.Webhooks),
		"emails", len(cfg.Emails),
	)

	if len(notifiers) == 0 {
		return dispatch.None{}, nil
	}

	return dispatch.New(notifiers), nil
}

// ValidateRuntimeTemplates renders configured notification templates with representative runtime events.
func ValidateRuntimeTemplates(
	templateFS fs.FS,
	cfg Config,
	checkInName string,
	expectedEvery time.Duration,
	alertingDelay time.Duration,
) error {
	alertingEvent, resolvedEvent := templateValidationEvents(checkInName, expectedEvery, alertingDelay)
	return validateTemplates(templateFS, cfg, alertingEvent, resolvedEvent)
}

// validateTemplates renders configured notification templates with representative events.
func validateTemplates(templateFS fs.FS, cfg Config, alertingEvent, resolvedEvent monitor.Event) error {
	if err := validateWebhookTemplates(templateFS, cfg.App, cfg.Webhooks, alertingEvent, resolvedEvent); err != nil {
		return err
	}
	if err := validateEmailTemplates(templateFS, cfg.App, cfg.Emails, alertingEvent, resolvedEvent); err != nil {
		return err
	}
	return nil
}

// templateValidationEvents builds realistic lifecycle events from runtime configuration.
func templateValidationEvents(
	checkInName string,
	expectedEvery time.Duration,
	alertingDelay time.Duration,
) (alertingEvent monitor.Event, resolvedEvent monitor.Event) {
	now := time.Now()
	// Pretend the monitor is exactly at the alert boundary so templates can exercise every timing field.
	lastCheckIn := now.Add(-expectedEvery - alertingDelay)
	expectedBy := lastCheckIn.Add(expectedEvery)
	alertingAt := expectedBy.Add(alertingDelay)

	alertingEvent = monitor.Event{
		CheckInName:  checkInName,
		LastCheckIn:  lastCheckIn,
		ExpectedBy:   expectedBy,
		OverdueSince: expectedBy,
		AlertingAt:   alertingAt,
		Now:          alertingAt,
		Phase:        monitor.PhaseAlerting,
		Status:       monitor.StatusAlerting,
		Resolved:     false,
	}
	resolvedEvent = alertingEvent
	resolvedEvent.Now = now
	resolvedEvent.Phase = monitor.PhaseAwaiting
	resolvedEvent.Status = monitor.StatusResolved
	resolvedEvent.Resolved = true
	return alertingEvent, resolvedEvent
}

// validateWebhookTemplates renders all webhook templates for configured webhook targets.
func validateWebhookTemplates(templateFS fs.FS, app render.AppData, configs []targets.WebhookConfig, alertingEvent, resolvedEvent monitor.Event) error {
	for _, cfg := range configs {
		cfg.ContentTemplates.App = app
		renderer, err := targets.NewWebhookRenderer(templateFS, cfg.Template, cfg.ContentTemplates)
		if err != nil {
			return err
		}
		if err := renderer.ValidateWithEvents(alertingEvent, resolvedEvent); err != nil {
			return fmt.Errorf("validate webhook %q templates: %w", cfg.Name, err)
		}
	}
	return nil
}

// validateEmailTemplates renders all email body and subject templates for configured email targets.
func validateEmailTemplates(templateFS fs.FS, app render.AppData, configs []targets.EmailConfig, alertingEvent, resolvedEvent monitor.Event) error {
	for _, cfg := range configs {
		cfg.ContentTemplates.App = app
		renderer, err := targets.NewEmailRenderer(
			templateFS,
			cfg.Template,
			cfg.SubjectTemplate,
			cfg.ResolvedSubjectTemplate,
			cfg.ContentTemplates,
		)
		if err != nil {
			return err
		}
		if err := renderer.ValidateWithEvents(alertingEvent, resolvedEvent); err != nil {
			return fmt.Errorf("validate email %q templates: %w", cfg.Name, err)
		}
	}
	return nil
}

// buildWebhookNotifiers builds webhook notifiers from typed webhook configuration.
func buildWebhookNotifiers(
	templateFS fs.FS,
	app render.AppData,
	configs []targets.WebhookConfig,
	logger *slog.Logger,
) (notifiers []delivery.Notifier, err error) {
	notifiers = make([]delivery.Notifier, 0, len(configs))

	for _, cfg := range configs {
		cfg.ContentTemplates.App = app
		renderer, err := targets.NewWebhookRenderer(templateFS, cfg.Template, cfg.ContentTemplates)
		if err != nil {
			return nil, err
		}

		notifiers = append(notifiers, targets.NewWebhook(
			cfg,
			renderer,
			logger.With("notifier", "webhook", "target", cfg.Name),
		))
	}

	return notifiers, nil
}

// buildEmailNotifiers builds email notifiers from typed email configuration.
func buildEmailNotifiers(
	templateFS fs.FS,
	app render.AppData,
	configs []targets.EmailConfig,
	logger *slog.Logger,
) (notifiers []delivery.Notifier, err error) {
	notifiers = make([]delivery.Notifier, 0, len(configs))

	for _, cfg := range configs {
		cfg.ContentTemplates.App = app
		renderer, err := targets.NewEmailRenderer(
			templateFS,
			cfg.Template,
			cfg.SubjectTemplate,
			cfg.ResolvedSubjectTemplate,
			cfg.ContentTemplates,
		)
		if err != nil {
			return nil, err
		}

		notifiers = append(notifiers, targets.NewEmail(
			cfg,
			renderer,
			logger.With("notifier", "email", "target", cfg.Name),
		))
	}

	return notifiers, nil
}
