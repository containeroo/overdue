package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContentRenderer(t *testing.T) {
	t.Parallel()

	t.Run("builds renderer with inline content and file body", func(t *testing.T) {
		t.Parallel()

		path := writeTemplate(t, `{{ .Title }} / {{ .Text }}`)

		renderer, err := NewContentRenderer(nil, path, ContentTemplates{
			Title:         `alerting {{ .CheckInName }}`,
			ResolvedTitle: `resolved {{ .CheckInName }}`,
			Text:          `down`,
			ResolvedText:  `up`,
		})

		require.NoError(t, err)
		require.NotNil(t, renderer.body)
		require.NotNil(t, renderer.title)
		require.NotNil(t, renderer.resolvedTitle)
		require.NotNil(t, renderer.text)
		require.NotNil(t, renderer.resolvedText)
	})

	t.Run("builds renderer with builtin body", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(testTemplateFS(), "builtin:body", DefaultContentTemplates())

		require.NoError(t, err)
		require.NotNil(t, renderer.body)
	})

	t.Run("returns content template parse errors", func(t *testing.T) {
		t.Parallel()

		_, err := NewContentRenderer(nil, "", ContentTemplates{Title: `{{ if }}`})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse title-template")
	})

	t.Run("returns body template read errors", func(t *testing.T) {
		t.Parallel()

		_, err := NewContentRenderer(nil, filepath.Join(t.TempDir(), "missing.tmpl"), DefaultContentTemplates())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read notification template")
	})

	t.Run("returns body template parse errors", func(t *testing.T) {
		t.Parallel()

		_, err := NewContentRenderer(nil, writeTemplate(t, `{{ if }}`), DefaultContentTemplates())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse notification template")
	})
}

func TestContentRendererEnrich(t *testing.T) {
	t.Parallel()

	t.Run("renders title and text", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(nil, "", ContentTemplates{
			Title: `alerting {{ .CheckInName }}`,
			Text:  `{{ .CheckInName }} is down`,
		})
		require.NoError(t, err)

		event, err := renderer.Enrich(monitor.Event{CheckInName: "prometheus"})

		require.NoError(t, err)
		assert.Equal(t, "alerting prometheus", event.Title)
		assert.Equal(t, "prometheus is down", event.Text)
	})

	t.Run("renders custom data", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(nil, writeTemplate(t, `{{ .Title }} / {{ .Text }} / {{ .CustomData.owner }}`), ContentTemplates{
			Title:      `alerting {{ .CustomData.owner }}`,
			Text:       `{{ .CheckInName }} is owned by {{ .CustomData.owner }}`,
			CustomData: map[string]string{"owner": "platform"},
		})
		require.NoError(t, err)

		event, err := renderer.Enrich(monitor.Event{CheckInName: "prometheus"})

		require.NoError(t, err)
		body, err := renderer.RenderBody(event)
		require.NoError(t, err)

		assert.Equal(t, "alerting platform", event.Title)
		assert.Equal(t, "prometheus is owned by platform", event.Text)
		assert.Equal(t, "alerting platform / prometheus is owned by platform / platform", body)
	})

	t.Run("renders app data", func(t *testing.T) {
		t.Parallel()

		app := AppData{
			Version:    "v0.0.7",
			PublicURL:  "https://overdue.example.test/overdue",
			CheckInURL: "https://overdue.example.test/overdue/check-in",
			StatusURL:  "https://overdue.example.test/overdue/status",
		}

		renderer, err := NewContentRenderer(nil, writeTemplate(t, `{{ .App.Version }} / {{ .App.CheckInURL }} / {{ .App.StatusURL }}`), ContentTemplates{
			Title: `overdue {{ .App.Version }}`,
			Text:  `status: {{ .App.StatusURL }}`,
			App:   app,
		})
		require.NoError(t, err)

		event, err := renderer.Enrich(monitor.Event{CheckInName: "prometheus"})

		require.NoError(t, err)
		body, err := renderer.RenderBody(event)
		require.NoError(t, err)

		assert.Equal(t, "overdue v0.0.7", event.Title)
		assert.Equal(t, "status: https://overdue.example.test/overdue/status", event.Text)
		assert.Equal(t, "v0.0.7 / https://overdue.example.test/overdue/check-in / https://overdue.example.test/overdue/status", body)
	})

	t.Run("renders resolved title and text", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(nil, "", ContentTemplates{
			Title:         `alerting`,
			ResolvedTitle: `resolved {{ .CheckInName }}`,
			Text:          `down`,
			ResolvedText:  `up`,
		})
		require.NoError(t, err)

		event, err := renderer.Enrich(monitor.Event{CheckInName: "prometheus", Status: monitor.StatusResolved})

		require.NoError(t, err)
		assert.Equal(t, "resolved prometheus", event.Title)
		assert.Equal(t, "up", event.Text)
	})

	t.Run("zero value uses defaults", func(t *testing.T) {
		t.Parallel()

		event, err := (ContentRenderer{}).Enrich(monitor.Event{CheckInName: "prometheus"})

		require.NoError(t, err)
		assert.Equal(t, defaultTitle, event.Title)
		assert.Equal(t, `Check-in "prometheus" is overdue:`, event.Text)
	})

	t.Run("returns render errors", func(t *testing.T) {
		t.Parallel()

		title, err := ParseInlineTemplate("title", `{{ .Missing.Field }}`)
		require.NoError(t, err)

		_, err = (ContentRenderer{
			body:          nil,
			title:         title,
			resolvedTitle: title,
			text:          title,
			resolvedText:  title,
		}).Enrich(monitor.Event{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render notification title")
	})
}

func TestContentRendererRenderBody(t *testing.T) {
	t.Parallel()

	t.Run("returns default body without configured body template", func(t *testing.T) {
		t.Parallel()

		body, err := (ContentRenderer{}).RenderBody(monitor.Event{CheckInName: "prometheus"})

		require.NoError(t, err)
		assert.Equal(t, `Check-in "prometheus" is overdue:`, body)
	})

	t.Run("renders configured body template", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(nil, writeTemplate(t, `{{ .Title }}: {{ .Text }}`), DefaultContentTemplates())
		require.NoError(t, err)

		body, err := renderer.RenderBody(monitor.Event{Title: "title", Text: "text"})

		require.NoError(t, err)
		assert.Equal(t, "title: text", body)
	})

	t.Run("returns execution errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(nil, writeTemplate(t, `{{ .Missing.Field }}`), DefaultContentTemplates())
		require.NoError(t, err)

		_, err = renderer.RenderBody(monitor.Event{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render notification template")
	})
}

func TestContentRendererWithDefaults(t *testing.T) {
	t.Parallel()

	t.Run("returns renderer unchanged when complete", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewContentRenderer(nil, "", DefaultContentTemplates())
		require.NoError(t, err)

		got, err := renderer.withDefaults()

		require.NoError(t, err)
		assert.Same(t, renderer.body, got.body)
		assert.Same(t, renderer.title, got.title)
		assert.Same(t, renderer.resolvedTitle, got.resolvedTitle)
		assert.Same(t, renderer.text, got.text)
		assert.Same(t, renderer.resolvedText, got.resolvedText)
	})

	t.Run("builds defaults for zero value", func(t *testing.T) {
		t.Parallel()

		got, err := (ContentRenderer{}).withDefaults()

		require.NoError(t, err)
		require.NotNil(t, got.title)
		require.NotNil(t, got.resolvedTitle)
		require.NotNil(t, got.text)
		require.NotNil(t, got.resolvedText)
	})
}

// writeTemplate writes a temporary notification body template.
func writeTemplate(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "template.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// testTemplateFS returns built-in template fixtures.
func testTemplateFS() fs.FS {
	return fstest.MapFS{
		"body.tmpl": {Data: []byte(`{{ .Title }}`)},
	}
}
