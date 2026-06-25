package webhook

import (
	"encoding/json"
	"testing"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRenderer(t *testing.T) {
	t.Parallel()
	t.Run("builds renderer", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(testTemplateFS(), "builtin:webhook", testContentTemplates())

		require.NoError(t, err)
		require.NotZero(t, renderer)
	})

	t.Run("returns content renderer errors", func(t *testing.T) {
		t.Parallel()

		_, err := NewRenderer(nil, "builtin:missing", render.DefaultContentTemplates())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "built-in templates are not configured")
	})
}

func TestRendererRenderBody(t *testing.T) {
	t.Parallel()
	t.Run("renders valid json body and enriched event", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(testTemplateFS(), "builtin:webhook", testContentTemplates())
		require.NoError(t, err)

		event, body, err := renderer.RenderBody(testEvent())

		require.NoError(t, err)
		assert.Equal(t, "alerting prometheus", event.Title)
		assert.Equal(t, "prometheus is down", event.Text)
		assert.JSONEq(t, `{"title":"alerting prometheus","text":"prometheus is down"}`, string(body))
	})

	t.Run("renders custom data", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `{"channel":{{ json .CustomData.channel }},"text":{{ json .Text }}}`), render.ContentTemplates{
			Text:       `{{ .CheckInName }} is down`,
			CustomData: map[string]string{"channel": "#ops"},
		})
		require.NoError(t, err)

		_, body, err := renderer.RenderBody(testEvent())

		require.NoError(t, err)
		assert.JSONEq(t, `{"channel":"#ops","text":"prometheus is down"}`, string(body))
	})

	t.Run("renders resolved content", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(testTemplateFS(), "builtin:webhook", testContentTemplates())
		require.NoError(t, err)

		event, body, err := renderer.RenderBody(testResolvedEvent())

		require.NoError(t, err)
		assert.Equal(t, "resolved prometheus", event.Title)
		assert.Equal(t, "prometheus is up", event.Text)
		assert.JSONEq(t, `{"title":"resolved prometheus","text":"prometheus is up"}`, string(body))
	})

	t.Run("rejects invalid json body", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `not json`), render.DefaultContentTemplates())
		require.NoError(t, err)

		_, _, err = renderer.RenderBody(testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "result is not valid JSON")
	})

	t.Run("returns enrichment errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `{}`), render.ContentTemplates{
			Title:         `{{ .Missing.Field }}`,
			ResolvedTitle: `resolved`,
			Text:          `text`,
			ResolvedText:  `resolved text`,
		})
		require.NoError(t, err)

		_, _, err = renderer.RenderBody(testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render notification title")
	})

	t.Run("returns body render errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `{{ .Missing.Field }}`), render.DefaultContentTemplates())
		require.NoError(t, err)

		_, _, err = renderer.RenderBody(testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render notification template")
	})
}

func TestRendererValidate(t *testing.T) {
	t.Parallel()
	t.Run("validates sample events", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(testTemplateFS(), "builtin:webhook", render.DefaultContentTemplates())
		require.NoError(t, err)

		err = renderer.Validate()

		require.NoError(t, err)
	})
}

func TestRendererValidateWithEvents(t *testing.T) {
	t.Parallel()
	t.Run("wraps alerting validation errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `not json`), render.DefaultContentTemplates())
		require.NoError(t, err)

		err = renderer.ValidateWithEvents(testEvent(), testResolvedEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate alerting webhook template")
	})

	t.Run("wraps resolved validation errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `{{ when .Resolved "not json" "{}" }}`), render.DefaultContentTemplates())
		require.NoError(t, err)

		err = renderer.ValidateWithEvents(testEvent(), testResolvedEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate resolved webhook template")
	})
}

func TestRendererJSONShape(t *testing.T) {
	t.Parallel()
	t.Run("body can be decoded by webhook receivers", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(testTemplateFS(), "builtin:webhook", testContentTemplates())
		require.NoError(t, err)

		_, body, err := renderer.RenderBody(monitor.Event{CheckInName: "prometheus"})
		require.NoError(t, err)

		var decoded map[string]string
		require.NoError(t, json.Unmarshal(body, &decoded))
		assert.Equal(t, "alerting prometheus", decoded["title"])
	})
}
