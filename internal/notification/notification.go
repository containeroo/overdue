package notification

import (
	"io/fs"
	"log/slog"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/dispatch"
	"github.com/containeroo/overdue/internal/notification/target"
)

// NewDispatcher creates the configured notification dispatcher.
func NewDispatcher(templateFS fs.FS, cfg config.Notifications, logger *slog.Logger) (target.Dispatcher, error) {
	if logger == nil {
		panic("notification logger must not be nil")
	}

	targets, err := buildTargets(templateFS, cfg, logger)
	if err != nil {
		return nil, err
	}

	logger.Info(
		"configured notifiers",
		"total", len(targets),
		"webhooks", len(cfg.Webhooks),
		"emails", len(cfg.Emails),
	)

	if len(targets) == 0 {
		return dispatch.None{}, nil
	}

	return dispatch.New(targets), nil
}

// ValidateTemplates renders configured notification templates with representative runtime events.
func ValidateTemplates(
	templateFS fs.FS,
	cfg config.Notifications,
	checkInName string,
	expectedEvery time.Duration,
	alertingDelay time.Duration,
) error {
	alertingEvent, resolvedEvent := templateValidationEvents(checkInName, expectedEvery, alertingDelay)
	return validateTemplates(templateFS, cfg, alertingEvent, resolvedEvent)
}

// templateValidationEvents builds realistic lifecycle events from runtime configuration.
func templateValidationEvents(
	checkInName string,
	expectedEvery time.Duration,
	alertingDelay time.Duration,
) (alertingEvent monitor.Event, resolvedEvent monitor.Event) {
	now := time.Now()
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
