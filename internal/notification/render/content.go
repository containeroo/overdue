package render

import (
	"fmt"
	"maps"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/utils"
)

const (
	defaultTitle         string = `[OVERDUE] Event Notification`
	defaultResolvedTitle string = `[RESOLVED] [OVERDUE] Event Notification`
	defaultText          string = `Check-in "{{ .CheckInName }}" is overdue:`
	defaultResolvedText  string = `Check-in "{{ .CheckInName }}" is resolved:`
)

// AppData contains application links exposed to notification templates.
type AppData struct {
	Version    string
	PublicURL  string
	CheckInURL string
	StatusURL  string
}

// NewAppData builds application template data from normalized application settings.
func NewAppData(version, publicURL, checkInPath string) AppData {
	app := AppData{Version: version}
	if publicURL == "" {
		return app
	}

	app.PublicURL = publicURL
	app.CheckInURL = publicURL + checkInPath
	app.StatusURL = publicURL + "/status"
	return app
}

// ContentTemplates configures title, text, body template data, and resolved event templates.
type ContentTemplates struct {
	Title         string
	ResolvedTitle string
	Text          string
	ResolvedText  string
	App           AppData
	CustomData    map[string]string
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

// Clone returns an independent copy of the content template configuration.
func (c ContentTemplates) Clone() ContentTemplates {
	c.CustomData = maps.Clone(c.CustomData)
	return c
}

// TemplateData is the value passed to notification templates.
//
// Event is embedded so existing templates can keep using fields such as
// .CheckInName, .Title, and .Status directly. App contains application links
// configured with public-url. CustomData contains target-local key/value pairs
// configured with custom-data flags.
type TemplateData struct {
	monitor.Event
	App        AppData
	CustomData map[string]string
}

// NewTemplateData builds template data from an event and target-local template data.
func NewTemplateData(event monitor.Event, app AppData, customData map[string]string) TemplateData {
	return TemplateData{
		Event:      event,
		App:        app,
		CustomData: maps.Clone(customData),
	}
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
