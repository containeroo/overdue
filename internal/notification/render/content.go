package render

import (
	"fmt"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/utils"
)

const (
	defaultTitle         string = `[OVERDUE] Event Notification`
	defaultResolvedTitle string = `[RESOLVED] [OVERDUE] Event Notification`
	defaultText          string = `Check-in "{{ .CheckInName }}" is overdue:`
	defaultResolvedText  string = `Check-in "{{ .CheckInName }}" is resolved:`
)

// ContentTemplates configures title and text templates for alerting and resolved events.
type ContentTemplates struct {
	Title         string
	ResolvedTitle string
	Text          string
	ResolvedText  string
}

// DefaultContentTemplates returns the built-in notification title and text templates.
func DefaultContentTemplates() ContentTemplates {
	return ContentTemplates{
		Title:         defaultTitle,
		ResolvedTitle: defaultResolvedTitle,
		Text:          defaultText,
		ResolvedText:  defaultResolvedText,
	}
}

// ApplyDefaults fills unset content templates with built-in defaults.
func (c *ContentTemplates) ApplyDefaults() {
	c.Title = utils.DefaultIfZero(c.Title, defaultTitle)
	c.ResolvedTitle = utils.DefaultIfZero(c.ResolvedTitle, defaultResolvedTitle)
	c.Text = utils.DefaultIfZero(c.Text, defaultText)
	c.ResolvedText = utils.DefaultIfZero(c.ResolvedText, defaultResolvedText)
}

// DefaultMessage returns the built-in notification message.
func DefaultMessage(event monitor.Event) string {
	if event.Text != "" {
		return event.Text
	}

	if event.Resolved || event.Status == monitor.StatusResolved {
		return fmt.Sprintf("Check-in %q is resolved:", event.CheckInName)
	}

	return fmt.Sprintf("Check-in %q is overdue:", event.CheckInName)
}
