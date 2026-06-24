package render

import (
	"testing"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
)

func TestDefaultContentTemplates(t *testing.T) {
	t.Parallel()
	t.Run("returns built in content templates", func(t *testing.T) {
		t.Parallel()

		templates := DefaultContentTemplates()

		assert.Equal(t, defaultTitle, templates.Title)
		assert.Equal(t, defaultResolvedTitle, templates.ResolvedTitle)
		assert.Equal(t, defaultText, templates.Text)
		assert.Equal(t, defaultResolvedText, templates.ResolvedText)
	})
}

func TestContentTemplatesApplyDefaults(t *testing.T) {
	t.Parallel()
	t.Run("fills only empty values", func(t *testing.T) {
		t.Parallel()

		templates := ContentTemplates{
			Title:        "custom title",
			Text:         "custom text",
			ResolvedText: "custom resolved text",
		}

		templates.ApplyDefaults()

		assert.Equal(t, "custom title", templates.Title)
		assert.Equal(t, defaultResolvedTitle, templates.ResolvedTitle)
		assert.Equal(t, "custom text", templates.Text)
		assert.Equal(t, "custom resolved text", templates.ResolvedText)
	})
}

func TestNewAppData(t *testing.T) {
	t.Parallel()

	t.Run("returns version without public url", func(t *testing.T) {
		t.Parallel()

		data := NewAppData("v0.0.7", "", "/check-in")

		assert.Equal(t, AppData{
			Version: "v0.0.7",
		}, data)
	})

	t.Run("builds public app links from normalized settings", func(t *testing.T) {
		t.Parallel()

		data := NewAppData("v0.0.7", "https://overdue.example.test/overdue", "/custom-check-in")

		assert.Equal(t, "v0.0.7", data.Version)
		assert.Equal(t, "https://overdue.example.test/overdue", data.PublicURL)
		assert.Equal(t, "https://overdue.example.test/overdue/custom-check-in", data.CheckInURL)
		assert.Equal(t, "https://overdue.example.test/overdue/status", data.StatusURL)
	})
}

func TestNewTemplateData(t *testing.T) {
	t.Parallel()

	t.Run("embeds event and clones custom data", func(t *testing.T) {
		t.Parallel()

		customData := map[string]string{"owner": "platform"}
		app := AppData{
			Version:    "v0.0.7",
			PublicURL:  "https://overdue.example.test",
			CheckInURL: "https://overdue.example.test/check-in",
			StatusURL:  "https://overdue.example.test/status",
		}

		data := NewTemplateData(monitor.Event{CheckInName: "prometheus"}, app, customData)
		customData["owner"] = "changed"

		assert.Equal(t, "prometheus", data.CheckInName)
		assert.Equal(t, app, data.App)
		assert.Equal(t, map[string]string{"owner": "platform"}, data.CustomData)
	})
}

func TestDefaultMessage(t *testing.T) {
	t.Parallel()

	t.Run("returns existing event text", func(t *testing.T) {
		t.Parallel()

		message := DefaultMessage(monitor.Event{Text: "already rendered"})

		assert.Equal(t, "already rendered", message)
	})

	t.Run("returns alerting message", func(t *testing.T) {
		t.Parallel()

		message := DefaultMessage(monitor.Event{CheckInName: "prometheus", Status: monitor.StatusAlerting})

		assert.Equal(t, `Check-in "prometheus" is overdue:`, message)
	})

	t.Run("returns resolved message when resolved flag is set", func(t *testing.T) {
		t.Parallel()

		message := DefaultMessage(monitor.Event{CheckInName: "prometheus", Resolved: true})

		assert.Equal(t, `Check-in "prometheus" is resolved:`, message)
	})

	t.Run("returns resolved message when status is resolved", func(t *testing.T) {
		t.Parallel()

		message := DefaultMessage(monitor.Event{CheckInName: "prometheus", Status: monitor.StatusResolved})

		assert.Equal(t, `Check-in "prometheus" is resolved:`, message)
	})
}
