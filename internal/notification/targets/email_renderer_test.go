package targets

import (
	"testing"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmailRenderer(t *testing.T) {
	t.Parallel()
	t.Run("builds renderer", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(
			testTemplateFS(),
			"builtin:email-html",
			`subject {{ .CheckInName }}`,
			`resolved {{ .CheckInName }}`,
			testContentTemplates(),
		)

		require.NoError(t, err)
		require.NotNil(t, renderer.subject)
		require.NotNil(t, renderer.resolvedSubject)
	})

	t.Run("returns content renderer errors", func(t *testing.T) {
		t.Parallel()

		_, err := NewEmailRenderer(nil, "builtin:missing", "", "", render.DefaultContentTemplates())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "built-in templates are not configured")
	})

	t.Run("returns subject parse errors", func(t *testing.T) {
		t.Parallel()

		_, err := NewEmailRenderer(nil, "", `{{ if }}`, "", render.DefaultContentTemplates())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse subject-template")
	})
}

func TestEmailRendererRender(t *testing.T) {
	t.Parallel()
	t.Run("renders alerting email message", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(
			testTemplateFS(),
			"builtin:email-html",
			`subject {{ .Title }}`,
			`resolved {{ .Title }}`,
			testContentTemplates(),
		)
		require.NoError(t, err)

		message, err := renderer.Render(testEvent())

		require.NoError(t, err)
		assert.Equal(t, "subject alerting prometheus", message.Subject)
		assert.Contains(t, message.Body, "alerting prometheus")
		assert.Contains(t, message.Body, "prometheus is down")
	})

	t.Run("renders resolved email message", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(
			testTemplateFS(),
			"builtin:email-html",
			`subject {{ .Title }}`,
			`resolved {{ .Title }}`,
			testContentTemplates(),
		)
		require.NoError(t, err)

		message, err := renderer.Render(testResolvedEvent())

		require.NoError(t, err)
		assert.Equal(t, "resolved resolved prometheus", message.Subject)
		assert.Contains(t, message.Body, "resolved prometheus")
		assert.Contains(t, message.Body, "prometheus is up")
	})

	t.Run("uses default subject templates", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(nil, "", "", "", testContentTemplates())
		require.NoError(t, err)

		message, err := renderer.Render(testEvent())

		require.NoError(t, err)
		assert.Equal(t, "alerting prometheus", message.Subject)
		assert.Equal(t, "prometheus is down", message.Body)
	})

	t.Run("returns body render errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(nil, writeTemplate(t, `{{ .Missing.Field }}`), "", "", render.DefaultContentTemplates())
		require.NoError(t, err)

		_, err = renderer.Render(testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render notification template")
	})

	t.Run("returns subject render errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(nil, "", `{{ .Missing.Field }}`, "", render.DefaultContentTemplates())
		require.NoError(t, err)

		_, err = renderer.Render(testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render email subject")
	})
}

func TestEmailRendererValidate(t *testing.T) {
	t.Parallel()
	t.Run("validates sample events", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(testTemplateFS(), "builtin:email-html", "", "", render.DefaultContentTemplates())
		require.NoError(t, err)

		err = renderer.Validate()

		require.NoError(t, err)
	})
}

func TestEmailRendererValidateWithEvents(t *testing.T) {
	t.Parallel()
	t.Run("wraps alerting validation errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(nil, "", `{{ .Missing.Field }}`, "", render.DefaultContentTemplates())
		require.NoError(t, err)

		err = renderer.ValidateWithEvents(testEvent(), testResolvedEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate alerting email template")
	})

	t.Run("wraps resolved validation errors", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(nil, "", `{{ .Title }}`, `{{ .Missing.Field }}`, render.DefaultContentTemplates())
		require.NoError(t, err)

		err = renderer.ValidateWithEvents(testEvent(), testResolvedEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate resolved email template")
	})
}

func TestEmailRendererRenderSubject(t *testing.T) {
	t.Parallel()
	t.Run("uses default subject for zero value renderer", func(t *testing.T) {
		t.Parallel()

		subject, err := (EmailRenderer{}).renderSubject(monitor.Event{Title: "default subject"})

		require.NoError(t, err)
		assert.Equal(t, "default subject", subject)
	})
}

func TestParseSubjectTemplates(t *testing.T) {
	t.Parallel()
	t.Run("applies defaults", func(t *testing.T) {
		t.Parallel()

		subject, resolvedSubject, err := parseSubjectTemplates("", "")

		require.NoError(t, err)
		require.NotNil(t, subject)
		require.NotNil(t, resolvedSubject)
	})

	t.Run("returns resolved subject parse errors", func(t *testing.T) {
		t.Parallel()

		_, _, err := parseSubjectTemplates("{{ .Title }}", `{{ if }}`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse resolved-subject-template")
	})
}
