package email

import (
	"fmt"
	"io/fs"
	"text/template"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
)

// Message contains the rendered email fields used by the SMTP notifier.
type Message struct {
	Subject string
	Body    string
}

// Renderer renders monitor events into email subjects and bodies.
type Renderer struct {
	content         render.ContentRenderer
	subject         *template.Template
	resolvedSubject *template.Template
}

// NewRenderer parses templates used to render email notifications.
func NewRenderer(
	templateFS fs.FS,
	source string,
	subject string,
	resolvedSubject string,
	content render.ContentTemplates,
) (Renderer, error) {
	contentRenderer, err := render.NewContentRenderer(templateFS, source, content)
	if err != nil {
		return Renderer{}, err
	}

	subjectTemplate, resolvedSubjectTemplate, err := parseSubjectTemplates(subject, resolvedSubject)
	if err != nil {
		return Renderer{}, err
	}

	return Renderer{
		content:         contentRenderer,
		subject:         subjectTemplate,
		resolvedSubject: resolvedSubjectTemplate,
	}, nil
}

// Render returns the rendered email subject and body for an event.
func (r Renderer) Render(event monitor.Event) (Message, error) {
	event, err := r.content.Enrich(event)
	if err != nil {
		return Message{}, err
	}

	body, err := r.content.RenderBody(event)
	if err != nil {
		return Message{}, err
	}

	subject, err := r.renderSubject(event)
	if err != nil {
		return Message{}, err
	}

	return Message{Subject: subject, Body: body}, nil
}

// Validate renders alerting and resolved email notifications to fail fast at startup.
func (r Renderer) Validate() error {
	return r.ValidateWithEvents(render.SampleAlertingEvent(), render.SampleResolvedEvent())
}

// ValidateWithEvents renders alerting and resolved email notifications with caller-supplied sample events.
func (r Renderer) ValidateWithEvents(alertingEvent, resolvedEvent monitor.Event) error {
	if _, err := r.Render(alertingEvent); err != nil {
		return fmt.Errorf("validate alerting email template: %w", err)
	}
	if _, err := r.Render(resolvedEvent); err != nil {
		return fmt.Errorf("validate resolved email template: %w", err)
	}
	return nil
}

// renderSubject renders the matching email subject for an event.
func (r Renderer) renderSubject(event monitor.Event) (subject string, err error) {
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

	subject, err = render.ExecuteInlineTemplate(subjectTemplate, r.content.TemplateData(event))
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
