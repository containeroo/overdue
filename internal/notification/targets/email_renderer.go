package targets

import (
	"fmt"
	"io/fs"
	"text/template"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
)

// EmailMessage contains the rendered email fields used by the SMTP notifier.
type EmailMessage struct {
	Subject string
	Body    string
}

// EmailRenderer renders monitor events into email subjects and bodies.
type EmailRenderer struct {
	content         render.ContentRenderer
	subject         *template.Template
	resolvedSubject *template.Template
}

// NewEmailRenderer parses templates used to render email notifications.
func NewEmailRenderer(
	templateFS fs.FS,
	source string,
	subject string,
	resolvedSubject string,
	content render.ContentTemplates,
) (EmailRenderer, error) {
	contentRenderer, err := render.NewContentRenderer(templateFS, source, content)
	if err != nil {
		return EmailRenderer{}, err
	}

	subjectTemplate, resolvedSubjectTemplate, err := parseSubjectTemplates(subject, resolvedSubject)
	if err != nil {
		return EmailRenderer{}, err
	}

	return EmailRenderer{
		content:         contentRenderer,
		subject:         subjectTemplate,
		resolvedSubject: resolvedSubjectTemplate,
	}, nil
}

// Render returns the rendered email subject and body for an event.
func (r EmailRenderer) Render(event monitor.Event) (EmailMessage, error) {
	event, err := r.content.Enrich(event)
	if err != nil {
		return EmailMessage{}, err
	}

	body, err := r.content.RenderBody(event)
	if err != nil {
		return EmailMessage{}, err
	}

	subject, err := r.renderSubject(event)
	if err != nil {
		return EmailMessage{}, err
	}

	return EmailMessage{Subject: subject, Body: body}, nil
}

// Validate renders alerting and resolved email notifications to fail fast at startup.
func (r EmailRenderer) Validate() error {
	return r.ValidateWithEvents(render.SampleAlertingEvent(), render.SampleResolvedEvent())
}

// ValidateWithEvents renders alerting and resolved email notifications with caller-supplied sample events.
func (r EmailRenderer) ValidateWithEvents(alertingEvent, resolvedEvent monitor.Event) error {
	if _, err := r.Render(alertingEvent); err != nil {
		return fmt.Errorf("validate alerting email template: %w", err)
	}
	if _, err := r.Render(resolvedEvent); err != nil {
		return fmt.Errorf("validate resolved email template: %w", err)
	}
	return nil
}

// renderSubject renders the matching email subject for an event.
func (r EmailRenderer) renderSubject(event monitor.Event) (subject string, err error) {
	subjectTemplate := r.subject
	if event.Resolved || event.Status == monitor.StatusResolved {
		subjectTemplate = r.resolvedSubject
	}

	if subjectTemplate == nil {
		subjectTemplate, _, err = parseSubjectTemplates("", "")
		if err != nil {
			return "", err
		}
	}

	subject, err = render.ExecuteInlineTemplate(subjectTemplate, event)
	if err != nil {
		return "", fmt.Errorf("render email subject: %w", err)
	}
	return subject, nil
}

// parseSubjectTemplates parses alerting and resolved email subject templates.
func parseSubjectTemplates(subject, resolvedSubject string) (subjectTemplate, resolvedSubjectTemplate *template.Template, err error) {
	if subject == "" {
		subject = "{{ .Title }}"
	}
	if resolvedSubject == "" {
		resolvedSubject = "{{ .Title }}"
	}

	subjectTemplate, err = render.ParseInlineTemplate("subject-template", subject)
	if err != nil {
		return nil, nil, err
	}
	resolvedSubjectTemplate, err = render.ParseInlineTemplate("resolved-subject-template", resolvedSubject)
	if err != nil {
		return nil, nil, err
	}
	return subjectTemplate, resolvedSubjectTemplate, nil
}
