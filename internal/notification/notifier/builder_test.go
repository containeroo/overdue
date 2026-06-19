package notifier

import (
	"maps"
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/dispatch"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactory(t *testing.T) {
	t.Parallel()
	t.Run("builds configured notifiers from typed config", func(t *testing.T) {
		t.Parallel()

		notifier, err := New(testTemplateFS(), Config{
			Webhooks: []targets.WebhookConfig{
				{
					Name:              "ops",
					URL:               "https://example.test/ops",
					Headers:           map[string]string{"Authorization": "Bearer token"},
					Timeout:           5 * time.Second,
					SkipInsecure:      true,
					SendResolved:      true,
					ContentTemplates:  render.DefaultContentTemplates(),
					Template:          "builtin:slack-incoming-webhook",
					LogResponse:       targets.LogResponseSummary,
					ResponseBodyLimit: 4096,
				},
			},
			Emails: []targets.EmailConfig{
				{
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
				},
			},
		}, testLogger())

		require.NoError(t, err)
		fanoutNotifier, ok := notifier.(*dispatch.Fanout)
		require.True(t, ok)
		require.Len(t, fanoutNotifier.Notifiers, 2)

		webhookNotifier, ok := fanoutNotifier.Notifiers[0].(targets.Webhook)
		require.True(t, ok)
		assert.Equal(t, "ops", webhookNotifier.Config().Name)
		assert.Equal(t, "https://example.test/ops", webhookNotifier.Config().URL)
		assert.True(t, webhookNotifier.Config().SendResolved)
		assert.True(t, maps.Equal(map[string]string{"Authorization": "Bearer token"}, webhookNotifier.Config().Headers))
		assertWebhookClient(t, webhookNotifier.Client(), 5*time.Second, true)

		emailNotifier, ok := fanoutNotifier.Notifiers[1].(targets.Email)
		require.True(t, ok)
		assert.Equal(t, "primary", emailNotifier.Config().Name)
		assert.Equal(t, "smtp.example.test", emailNotifier.Config().Host)
		assert.True(t, emailNotifier.Config().SendResolved)
	})

	t.Run("validates invalid webhook template json", func(t *testing.T) {
		t.Parallel()

		err := validateTemplates(
			testTemplateFS(),
			Config{Webhooks: []targets.WebhookConfig{{Name: "ops", Template: writeTemplate(t, `not json`)}}},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate webhook")
	})

	t.Run("validates invalid title template", func(t *testing.T) {
		t.Parallel()

		defaults := render.DefaultContentTemplates()
		err := validateTemplates(
			testTemplateFS(),
			Config{Webhooks: []targets.WebhookConfig{{
				Name:     "ops",
				Template: writeTemplate(t, `{"text":{{ json .Text }}}`),
				ContentTemplates: render.ContentTemplates{
					Title:         "{{ .Missing.Field }}",
					ResolvedTitle: defaults.ResolvedTitle,
					Text:          defaults.Text,
					ResolvedText:  defaults.ResolvedText,
				},
			}}},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate webhook")
	})

	t.Run("validates builtin templates", func(t *testing.T) {
		t.Parallel()

		err := validateTemplates(
			testTemplateFS(),
			Config{
				Webhooks: []targets.WebhookConfig{{Name: "ops", Template: "builtin:slack-incoming-webhook", ContentTemplates: render.DefaultContentTemplates()}},
				Emails: []targets.EmailConfig{{
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

	t.Run("validates with caller supplied event", func(t *testing.T) {
		t.Parallel()

		defaults := render.DefaultContentTemplates()
		alertingEvent := render.SampleAlertingEvent()
		alertingEvent.CheckInName = "prometheus"
		resolvedEvent := render.SampleResolvedEvent()
		resolvedEvent.CheckInName = "prometheus"

		err := validateTemplates(
			testTemplateFS(),
			Config{Webhooks: []targets.WebhookConfig{{
				Name:     "ops",
				Template: writeTemplate(t, `{"text":{{ json .Text }}}`),
				ContentTemplates: render.ContentTemplates{
					Title:         `{{ if ne .CheckInName "prometheus" }}{{ .Missing.Field }}{{ end }}`,
					ResolvedTitle: defaults.ResolvedTitle,
					Text:          defaults.Text,
					ResolvedText:  defaults.ResolvedText,
				},
			}}},
			alertingEvent,
			resolvedEvent,
		)

		require.NoError(t, err)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("returns none without notifiers", func(t *testing.T) {
		t.Parallel()

		notifier, err := New(testTemplateFS(), Config{}, testLogger())

		require.NoError(t, err)
		assert.IsType(t, dispatch.None{}, notifier)
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "notification setup logger must not be nil", func() {
			_, _ = New(testTemplateFS(), Config{}, nil)
		})
	})
}

func TestValidateRuntimeTemplates(t *testing.T) {
	t.Parallel()
	t.Run("uses runtime check in name and timing data", func(t *testing.T) {
		t.Parallel()

		defaults := render.DefaultContentTemplates()
		err := ValidateRuntimeTemplates(
			testTemplateFS(),
			Config{Webhooks: []targets.WebhookConfig{{
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

		err := ValidateRuntimeTemplates(
			testTemplateFS(),
			Config{Emails: []targets.EmailConfig{{
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
		assert.Contains(t, err.Error(), "validate email")
	})
}

func TestValidateTemplates(t *testing.T) {
	t.Parallel()
	t.Run("returns webhook validation errors first", func(t *testing.T) {
		t.Parallel()

		err := validateTemplates(
			testTemplateFS(),
			Config{
				Webhooks: []targets.WebhookConfig{{Name: "ops", Template: writeTemplate(t, `not json`)}},
				Emails:   []targets.EmailConfig{{Name: "primary", Template: "builtin:missing"}},
			},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate webhook")
	})

	t.Run("returns email validation errors", func(t *testing.T) {
		t.Parallel()

		err := validateTemplates(
			testTemplateFS(),
			Config{Emails: []targets.EmailConfig{{Name: "primary", Template: "builtin:missing"}}},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
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

func TestValidateWebhookTemplates(t *testing.T) {
	t.Parallel()
	t.Run("returns renderer construction errors", func(t *testing.T) {
		t.Parallel()

		err := validateWebhookTemplates(
			testTemplateFS(),
			[]targets.WebhookConfig{{Name: "ops", Template: "builtin:missing"}},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
	})
}

func TestValidateEmailTemplates(t *testing.T) {
	t.Parallel()
	t.Run("returns renderer construction errors", func(t *testing.T) {
		t.Parallel()

		err := validateEmailTemplates(
			testTemplateFS(),
			[]targets.EmailConfig{{Name: "primary", Template: "builtin:missing"}},
			render.SampleAlertingEvent(),
			render.SampleResolvedEvent(),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
	})
}

func TestBuildWebhookNotifiers(t *testing.T) {
	t.Parallel()
	t.Run("returns empty slice without configs", func(t *testing.T) {
		t.Parallel()

		notifiers, err := buildWebhookNotifiers(testTemplateFS(), nil, testLogger())

		require.NoError(t, err)
		assert.Len(t, notifiers, 0)
	})

	t.Run("returns renderer errors", func(t *testing.T) {
		t.Parallel()

		notifiers, err := buildWebhookNotifiers(testTemplateFS(), []targets.WebhookConfig{{Name: "ops", Template: "builtin:missing"}}, testLogger())

		require.Error(t, err)
		assert.Nil(t, notifiers)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
	})
}

func TestBuildEmailNotifiers(t *testing.T) {
	t.Parallel()
	t.Run("returns empty slice without configs", func(t *testing.T) {
		t.Parallel()

		notifiers, err := buildEmailNotifiers(testTemplateFS(), nil, testLogger())

		require.NoError(t, err)
		assert.Len(t, notifiers, 0)
	})

	t.Run("returns renderer errors", func(t *testing.T) {
		t.Parallel()

		notifiers, err := buildEmailNotifiers(testTemplateFS(), []targets.EmailConfig{{Name: "primary", Template: "builtin:missing"}}, testLogger())

		require.Error(t, err)
		assert.Nil(t, notifiers)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
	})
}
