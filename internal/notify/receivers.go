package notify

import (
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/monitor"

	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/notifykit/targets/email"
	"github.com/containeroo/notifykit/targets/webhook"
	"github.com/containeroo/notifykit/templates"
)

const defaultSubjectTemplate = `{{ .Title }}`

// ReceiversFromConfig builds notifykit receivers and routing policy from Overdue notification config.
func ReceiversFromConfig(templateFS fs.FS, cfg config.Notifications, logger *slog.Logger) (kit.Receivers, *Router, error) {
	receivers := make(kit.Receivers, len(cfg.Webhooks)+len(cfg.Emails))
	resolvedReceivers := make([]kit.ReceiverID, 0, len(cfg.Webhooks)+len(cfg.Emails))

	for _, webhookCfg := range cfg.Webhooks {
		receiver, err := webhookReceiver(templateFS, cfg.App, webhookCfg, logger)
		if err != nil {
			return nil, nil, err
		}
		receivers[receiver.ID] = receiver
		if webhookCfg.SendResolved {
			resolvedReceivers = append(resolvedReceivers, receiver.ID)
		}
	}

	for _, emailCfg := range cfg.Emails {
		receiver, err := emailReceiver(templateFS, cfg.App, emailCfg)
		if err != nil {
			return nil, nil, err
		}
		receivers[receiver.ID] = receiver
		if emailCfg.SendResolved {
			resolvedReceivers = append(resolvedReceivers, receiver.ID)
		}
	}

	return receivers, NewRouter(resolvedReceivers), nil
}

// Router owns notification receiver routing rules.
type Router struct {
	resolvedReceivers []kit.ReceiverID
}

// NewRouter creates a notification router.
func NewRouter(resolvedReceivers []kit.ReceiverID) *Router {
	return &Router{
		resolvedReceivers: append([]kit.ReceiverID(nil), resolvedReceivers...),
	}
}

// ReceiverIDsForEvent returns explicit receiver routing for an event.
func (r *Router) ReceiverIDsForEvent(event monitor.Event) ([]kit.ReceiverID, bool) {
	if !event.Resolved {
		return nil, true
	}
	if r == nil || len(r.resolvedReceivers) == 0 {
		return nil, false
	}
	return append([]kit.ReceiverID(nil), r.resolvedReceivers...), true
}

// webhookReceiver creates one webhook receiver from config.
func webhookReceiver(templateFS fs.FS, app config.AppData, cfg config.WebhookConfig, logger *slog.Logger) (*kit.Receiver, error) {
	tmpl, err := templates.LoadSource(templateFS, cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("load webhook %q template: %w", cfg.Name, err)
	}

	subjectTmpl, err := templates.ParseStringTemplate("webhook-subject", subjectTemplate(cfg.SubjectTemplate))
	if err != nil {
		return nil, fmt.Errorf("parse webhook %q subject template: %w", cfg.Name, err)
	}

	clientOptions := []webhook.ClientOption{webhook.WithProxyFromEnvironment()}
	if cfg.TLSSkipVerify {
		clientOptions = append(clientOptions, webhook.WithSkipTLSVerify())
	}

	target := webhook.NewFromTarget(
		webhook.Target{
			Name:              cfg.Name,
			URL:               cfg.URL,
			Method:            cfg.Method,
			Headers:           maps.Clone(cfg.Headers),
			Template:          tmpl,
			SubjectTmpl:       subjectTmpl,
			ValidateJSON:      true,
			LogResponse:       webhookLogResponse(cfg.LogResponse),
			ResponseBodyLimit: cfg.ResponseBodyLimit,
		},
		webhook.WithClient(webhook.NewClient(cfg.Timeout, clientOptions...)),
		webhook.WithLogger(logger),
	)

	vars := varsFromConfig(app, cfg.CustomData)
	if err := validateWebhookTarget(target, cfg.Name, vars); err != nil {
		return nil, err
	}

	return &kit.Receiver{
		ID:      webhookReceiverID(cfg.Name),
		Name:    cfg.Name,
		Targets: []kit.Target{target},
		Vars:    vars,
	}, nil
}

// emailReceiver creates one email receiver from config.
func emailReceiver(templateFS fs.FS, app config.AppData, cfg config.EmailConfig) (*kit.Receiver, error) {
	tmpl, err := templates.LoadSource(templateFS, cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("load email %q template: %w", cfg.Name, err)
	}

	subjectTmpl, err := templates.ParseStringTemplate("email-subject", subjectTemplate(cfg.SubjectTemplate))
	if err != nil {
		return nil, fmt.Errorf("parse email %q subject template: %w", cfg.Name, err)
	}

	target := email.NewFromTarget(email.Target{
		Name:          cfg.Name,
		Host:          cfg.Host,
		Port:          cfg.Port,
		User:          cfg.User,
		Pass:          cfg.Pass,
		From:          cfg.From,
		To:            append([]string(nil), cfg.To...),
		Headers:       maps.Clone(cfg.Headers),
		SkipTLSVerify: cfg.SMTPTLSSkipVerify,
		Template:      tmpl,
		SubjectTmpl:   subjectTmpl,
	})

	vars := varsFromConfig(app, cfg.CustomData)
	if err := validateEmailTarget(target, cfg.Name, vars); err != nil {
		return nil, err
	}

	return &kit.Receiver{
		ID:      emailReceiverID(cfg.Name),
		Name:    cfg.Name,
		Targets: []kit.Target{target},
		Vars:    vars,
	}, nil
}

// webhookReceiverID returns the stable notifykit receiver ID for a webhook target.
func webhookReceiverID(name string) kit.ReceiverID {
	return kit.ReceiverID("webhook." + name)
}

// emailReceiverID returns the stable notifykit receiver ID for an email target.
func emailReceiverID(name string) kit.ReceiverID {
	return kit.ReceiverID("email." + name)
}

// subjectTemplate returns the configured subject template or a renderer-owned default.
func subjectTemplate(value string) string {
	if value == "" {
		return defaultSubjectTemplate
	}
	return value
}

// webhookLogResponse converts normalized config into the notifykit webhook type.
func webhookLogResponse(value config.WebhookLogResponse) webhook.LogResponse {
	switch value {
	case config.WebhookLogResponseBody:
		return webhook.LogResponseBody
	case config.WebhookLogResponseFull:
		return webhook.LogResponseFull
	case config.WebhookLogResponseNone:
		return webhook.LogResponseNone
	case config.WebhookLogResponseSummary, "":
		return webhook.LogResponseSummary
	default:
		return webhook.LogResponse(value)
	}
}

// validateWebhookTarget renders sample alerting and resolved webhook payloads.
func validateWebhookTarget(target *webhook.Target, receiverName string, vars map[string]any) error {
	alertingEvent, resolvedEvent := validationEvents("default")

	if err := target.Validate(kit.Payload{
		Notification: NewEvent(alertingEvent, nil),
		Receiver:     receiverName,
		Vars:         vars,
	}); err != nil {
		return fmt.Errorf("validate webhook %q alerting template: %w", receiverName, err)
	}

	if err := target.Validate(kit.Payload{
		Notification: NewEvent(resolvedEvent, nil),
		Receiver:     receiverName,
		Vars:         vars,
	}); err != nil {
		return fmt.Errorf("validate webhook %q resolved template: %w", receiverName, err)
	}

	return nil
}

// validateEmailTarget renders sample alerting and resolved email payloads.
func validateEmailTarget(target *email.Target, receiverName string, vars map[string]any) error {
	alertingEvent, resolvedEvent := validationEvents("default")

	if err := target.Validate(kit.Payload{
		Notification: NewEvent(alertingEvent, nil),
		Receiver:     receiverName,
		Vars:         vars,
	}); err != nil {
		return fmt.Errorf("validate email %q alerting template: %w", receiverName, err)
	}

	if err := target.Validate(kit.Payload{
		Notification: NewEvent(resolvedEvent, nil),
		Receiver:     receiverName,
		Vars:         vars,
	}); err != nil {
		return fmt.Errorf("validate email %q resolved template: %w", receiverName, err)
	}

	return nil
}

// validationEvents returns representative alerting and resolved events for template checks.
func validationEvents(checkInName string) (alertingEvent monitor.Event, resolvedEvent monitor.Event) {
	now := time.Now()
	lastCheckIn := now.Add(-8 * time.Second)
	expectedBy := lastCheckIn.Add(5 * time.Second)
	alertingAt := expectedBy.Add(3 * time.Second)

	alertingEvent = monitor.Event{
		IncidentID:     "validation-incident",
		NotificationID: "validation-alerting",
		CheckInName:    checkInName,
		LastCheckIn:    lastCheckIn,
		ExpectedBy:     expectedBy,
		OverdueSince:   expectedBy,
		AlertingAt:     alertingAt,
		Now:            alertingAt,
		Phase:          monitor.PhaseAlerting,
		Status:         monitor.StatusAlerting,
	}

	resolvedEvent = alertingEvent
	resolvedEvent.NotificationID = "validation-resolved"
	resolvedEvent.Now = now
	resolvedEvent.Phase = monitor.PhaseAwaiting
	resolvedEvent.Status = monitor.StatusResolved
	resolvedEvent.Resolved = true

	return alertingEvent, resolvedEvent
}
