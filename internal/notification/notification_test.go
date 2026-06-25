package notification

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/dispatch"
	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDispatcher(t *testing.T) {
	t.Parallel()

	t.Run("returns none without targets", func(t *testing.T) {
		t.Parallel()

		dispatcher, err := NewDispatcher(testTemplateFS(), config.Notifications{}, testLogger())

		require.NoError(t, err)
		assert.IsType(t, dispatch.None{}, dispatcher)
	})

	t.Run("builds configured targets in family order", func(t *testing.T) {
		t.Parallel()

		cfg := config.Notifications{
			App: render.AppData{Version: "1.2.3", PublicURL: "https://overdue.example.test"},
			Webhooks: []webhook.Config{{
				Name:              "ops",
				URL:               "https://example.test/ops",
				Headers:           map[string]string{"Authorization": "Bearer token"},
				Timeout:           5 * time.Second,
				SkipInsecure:      true,
				SendResolved:      true,
				ContentTemplates:  render.DefaultContentTemplates(),
				Template:          "builtin:slack-incoming-webhook",
				LogResponse:       webhook.LogResponseSummary,
				ResponseBodyLimit: 4096,
			}},
			Emails: []email.Config{{
				Name:                    "primary",
				Host:                    "smtp.example.test",
				Port:                    587,
				User:                    "user",
				Pass:                    "pass",
				SendResolved:            true,
				From:                    "overdue@example.test",
				To:                      []string{"ops@example.test"},
				SubjectTemplate:         "{{ .Title }}",
				ResolvedSubjectTemplate: "{{ .Title }}",
				ContentTemplates:        render.DefaultContentTemplates(),
				Template:                "builtin:email-html",
			}},
		}

		dispatcher, err := NewDispatcher(testTemplateFS(), cfg, testLogger())

		require.NoError(t, err)
		fanout, ok := dispatcher.(*dispatch.Fanout)
		require.True(t, ok)
		assert.Equal(t, []struct{ Type, Name string }{{"webhook", "ops"}, {"email", "primary"}}, targetPairs(fanout))
	})

	t.Run("wraps target build errors", func(t *testing.T) {
		t.Parallel()

		dispatcher, err := NewDispatcher(testTemplateFS(), config.Notifications{
			Webhooks: []webhook.Config{{Name: "ops", Template: "builtin:missing", Timeout: time.Second}},
		}, testLogger())

		require.Error(t, err)
		assert.Nil(t, dispatcher)
		assert.Contains(t, err.Error(), `build webhook "ops" renderer`)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "notification logger must not be nil", func() {
			_, _ = NewDispatcher(testTemplateFS(), config.Notifications{}, nil)
		})
	})
}

func TestValidateTemplates(t *testing.T) {
	t.Parallel()

	t.Run("uses runtime check in name and timing data", func(t *testing.T) {
		t.Parallel()

		defaults := render.DefaultContentTemplates()
		err := ValidateTemplates(
			testTemplateFS(),
			config.Notifications{Webhooks: []webhook.Config{{
				Name:     "ops",
				Template: writeTemplate(t, `{"overdueFor":{{ json (duration (.Now.Sub .ExpectedBy)) }},"text":{{ json .Text }}}`),
				ContentTemplates: render.ContentTemplates{
					Title:         `{{ if ne .CheckInName "prometheus" }}{{ .Missing.Field }}{{ end }}`,
					ResolvedTitle: defaults.ResolvedTitle,
					Text:          `{{ .CheckInName }} overdue after {{ duration (.Now.Sub .ExpectedBy) }}`,
					ResolvedText:  defaults.ResolvedText,
				},
			}}},
			"prometheus",
			5*time.Second,
			3*time.Second,
		)

		require.NoError(t, err)
	})

	t.Run("returns validation errors", func(t *testing.T) {
		t.Parallel()

		err := ValidateTemplates(
			testTemplateFS(),
			config.Notifications{Emails: []email.Config{{
				Name:                    "primary",
				Template:                "builtin:email-html",
				SubjectTemplate:         `{{ .Title }}`,
				ResolvedSubjectTemplate: `{{ .Missing.Field }}`,
				ContentTemplates:        render.DefaultContentTemplates(),
			}}},
			"prometheus",
			5*time.Second,
			3*time.Second,
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `validate email "primary" templates`)
	})
}

func TestValidateTemplatesInternal(t *testing.T) {
	t.Parallel()

	t.Run("returns webhook validation errors before email errors", func(t *testing.T) {
		t.Parallel()

		err := validateTemplates(
			testTemplateFS(),
			config.Notifications{
				Webhooks: []webhook.Config{{Name: "ops", Template: writeTemplate(t, `not json`)}},
				Emails:   []email.Config{{Name: "primary", Template: "builtin:missing"}},
			},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `validate webhook "ops" templates`)
	})

	t.Run("validates builtin templates", func(t *testing.T) {
		t.Parallel()

		err := validateTemplates(
			testTemplateFS(),
			config.Notifications{
				Webhooks: []webhook.Config{{Name: "ops", Template: "builtin:slack-incoming-webhook", ContentTemplates: render.DefaultContentTemplates()}},
				Emails: []email.Config{{
					Name:                    "primary",
					SubjectTemplate:         "{{ .Title }}",
					ResolvedSubjectTemplate: "{{ .Title }}",
					Template:                "builtin:email-html",
					ContentTemplates:        render.DefaultContentTemplates(),
				}},
			},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.NoError(t, err)
	})
}

func TestTemplateValidationEvents(t *testing.T) {
	t.Parallel()

	t.Run("builds matching alerting and resolved lifecycle events", func(t *testing.T) {
		t.Parallel()

		alertingEvent, resolvedEvent := templateValidationEvents("prometheus", 5*time.Second, 3*time.Second)

		assert.Equal(t, "prometheus", alertingEvent.CheckInName)
		assert.Equal(t, monitor.PhaseAlerting, alertingEvent.Phase)
		assert.Equal(t, monitor.StatusAlerting, alertingEvent.Status)
		assert.False(t, alertingEvent.Resolved)
		assert.True(t, alertingEvent.ExpectedBy.Equal(alertingEvent.LastCheckIn.Add(5*time.Second)))
		assert.True(t, alertingEvent.AlertingAt.Equal(alertingEvent.ExpectedBy.Add(3*time.Second)))
		assert.True(t, alertingEvent.Now.Equal(alertingEvent.AlertingAt))

		assert.Equal(t, alertingEvent.CheckInName, resolvedEvent.CheckInName)
		assert.True(t, resolvedEvent.LastCheckIn.Equal(alertingEvent.LastCheckIn))
		assert.True(t, resolvedEvent.ExpectedBy.Equal(alertingEvent.ExpectedBy))
		assert.True(t, resolvedEvent.AlertingAt.Equal(alertingEvent.AlertingAt))
		assert.Equal(t, monitor.PhaseAwaiting, resolvedEvent.Phase)
		assert.Equal(t, monitor.StatusResolved, resolvedEvent.Status)
		assert.True(t, resolvedEvent.Resolved)
		assert.True(t, resolvedEvent.Now.Equal(alertingEvent.Now))
	})
}

func targetPairs(fanout *dispatch.Fanout) []struct{ Type, Name string } {
	targets := fanout.Targets()
	out := make([]struct{ Type, Name string }, 0, len(targets))
	for _, target := range targets {
		out = append(out, struct{ Type, Name string }{target.Type, target.Name})
	}
	return out
}
