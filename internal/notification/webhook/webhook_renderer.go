package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
)

// Renderer renders monitor events into complete JSON webhook request bodies.
type Renderer struct {
	content render.ContentRenderer
}

// NewRenderer parses templates used to render webhook notifications.
func NewRenderer(templateFS fs.FS, source string, content render.ContentTemplates) (Renderer, error) {
	renderer, err := render.NewContentRenderer(templateFS, source, content)
	if err != nil {
		return Renderer{}, err
	}
	return Renderer{content: renderer}, nil
}

// RenderBody returns a complete JSON webhook request body.
func (r Renderer) RenderBody(event monitor.Event) (enriched monitor.Event, body []byte, err error) {
	enriched, err = r.content.Enrich(event)
	if err != nil {
		return monitor.Event{}, nil, err
	}

	text, err := r.content.RenderBody(enriched)
	if err != nil {
		return monitor.Event{}, nil, err
	}

	body = bytes.TrimSpace([]byte(text))
	if !json.Valid(body) {
		return monitor.Event{}, nil, fmt.Errorf("render webhook template: result is not valid JSON")
	}
	return enriched, body, nil
}

// Validate renders alerting and resolved webhook notifications and validates JSON.
func (r Renderer) Validate() error {
	return r.ValidateWithEvents(render.SampleAlertingEvent(), render.SampleResolvedEvent())
}

// ValidateWithEvents renders alerting and resolved webhook notifications with caller-supplied sample events.
func (r Renderer) ValidateWithEvents(alertingEvent, resolvedEvent monitor.Event) error {
	if _, _, err := r.RenderBody(alertingEvent); err != nil {
		return fmt.Errorf("validate alerting webhook template: %w", err)
	}
	if _, _, err := r.RenderBody(resolvedEvent); err != nil {
		return fmt.Errorf("validate resolved webhook template: %w", err)
	}
	return nil
}
