package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/templates"
)

// ContentRenderer applies shared notification title, text, and body templates.
// Transport-specific renderers wrap it to produce webhook or email payloads.
type ContentRenderer struct {
	body          *template.Template
	title         *template.Template
	resolvedTitle *template.Template
	text          *template.Template
	resolvedText  *template.Template
}

// NewContentRenderer parses the shared notification title, text, and body templates.
func NewContentRenderer(templateFS fs.FS, source string, content ContentTemplates) (renderer ContentRenderer, err error) {
	content.ApplyDefaults()

	title, err := ParseInlineTemplate("title-template", content.Title)
	if err != nil {
		return ContentRenderer{}, err
	}

	resolvedTitle, err := ParseInlineTemplate("resolved-title-template", content.ResolvedTitle)
	if err != nil {
		return ContentRenderer{}, err
	}

	text, err := ParseInlineTemplate("text-template", content.Text)
	if err != nil {
		return ContentRenderer{}, err
	}

	resolvedText, err := ParseInlineTemplate("resolved-text-template", content.ResolvedText)
	if err != nil {
		return ContentRenderer{}, err
	}

	var body *template.Template
	if strings.TrimSpace(source) != "" {
		body, err = parseBodyTemplate(templateFS, source)
		if err != nil {
			return ContentRenderer{}, err
		}
	}

	return ContentRenderer{
		body:          body,
		title:         title,
		resolvedTitle: resolvedTitle,
		text:          text,
		resolvedText:  resolvedText,
	}, nil
}

// parseBodyTemplate parses a notification body template from a file path or builtin reference.
func parseBodyTemplate(templateFS fs.FS, source string) (*template.Template, error) {
	name, body, err := readBodyTemplate(templateFS, source)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse notification template %q: %w", source, err)
	}
	return tmpl, nil
}

// readBodyTemplate reads a notification body template from a file path or builtin reference.
func readBodyTemplate(templateFS fs.FS, source string) (name string, body string, err error) {
	source = strings.TrimSpace(source)
	if templates.IsBuiltin(source) {
		return templates.New(templateFS).Read(source)
	}

	bodyBytes, err := os.ReadFile(source)
	if err != nil {
		return "", "", fmt.Errorf("read notification template %q: %w", source, err)
	}
	return filepath.Base(source), string(bodyBytes), nil
}

// Enrich returns an event copy with title and text templates applied.
func (r ContentRenderer) Enrich(event monitor.Event) (enriched monitor.Event, err error) {
	r, err = r.withDefaults()
	if err != nil {
		return monitor.Event{}, err
	}

	titleTemplate := r.title
	textTemplate := r.text
	if event.Resolved || event.Status == monitor.StatusResolved {
		titleTemplate = r.resolvedTitle
		textTemplate = r.resolvedText
	}

	title, err := ExecuteInlineTemplate(titleTemplate, event)
	if err != nil {
		return monitor.Event{}, fmt.Errorf("render notification title: %w", err)
	}
	text, err := ExecuteInlineTemplate(textTemplate, event)
	if err != nil {
		return monitor.Event{}, fmt.Errorf("render notification text: %w", err)
	}

	event.Title = title
	event.Text = text
	return event, nil
}

// RenderBody renders the configured body template or the built-in default body.
func (r ContentRenderer) RenderBody(event monitor.Event) (text string, err error) {
	r, err = r.withDefaults()
	if err != nil {
		return "", err
	}

	if r.body == nil {
		return DefaultMessage(event), nil
	}

	var buf bytes.Buffer
	if err := r.body.Execute(&buf, event); err != nil {
		return "", fmt.Errorf("render notification template: %w", err)
	}
	return buf.String(), nil
}

// withDefaults makes the zero value usable for tests and defensive construction.
func (r ContentRenderer) withDefaults() (ContentRenderer, error) {
	if r.title != nil && r.resolvedTitle != nil && r.text != nil && r.resolvedText != nil {
		return r, nil
	}
	return NewContentRenderer(nil, "", DefaultContentTemplates())
}
