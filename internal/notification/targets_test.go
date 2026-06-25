package notification

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationFamilies(t *testing.T) {
	t.Parallel()

	t.Run("keeps webhook targets before email targets", func(t *testing.T) {
		t.Parallel()

		families := notificationFamilies(config.Notifications{
			Webhooks: []webhook.Config{{Name: "ops"}},
			Emails:   []email.Config{{Name: "primary"}},
		})

		require.Len(t, families, 2)
		assert.IsType(t, webhookFamily{}, families[0])
		assert.IsType(t, emailFamily{}, families[1])
		assert.Equal(t, 2, targetCount(families))
	})
}

func TestBuildTargets(t *testing.T) {
	t.Parallel()

	t.Run("builds target notifiers", func(t *testing.T) {
		t.Parallel()

		built, err := buildTargets(
			testTemplateFS(),
			config.Notifications{
				App: render.AppData{Version: "dev"},
				Webhooks: []webhook.Config{{
					Name:              "ops",
					URL:               "https://example.test/webhook",
					Timeout:           time.Second,
					Template:          "builtin:slack-incoming-webhook",
					ContentTemplates:  render.DefaultContentTemplates(),
					ResponseBodyLimit: 4096,
				}},
				Emails: []email.Config{{
					Name:             "primary",
					Template:         "builtin:email-html",
					ContentTemplates: render.DefaultContentTemplates(),
				}},
			},
			testLogger(),
		)

		require.NoError(t, err)
		require.Len(t, built, 2)
		assert.Equal(t, "webhook", built[0].Target().Type)
		assert.Equal(t, "ops", built[0].Target().Name)
		assert.Equal(t, "email", built[1].Target().Type)
		assert.Equal(t, "primary", built[1].Target().Name)
	})

	t.Run("returns first family error", func(t *testing.T) {
		t.Parallel()

		built, err := buildTargets(
			testTemplateFS(),
			config.Notifications{Webhooks: []webhook.Config{{Name: "ops", Template: "builtin:missing", Timeout: time.Second}}},
			testLogger(),
		)

		require.Error(t, err)
		assert.Nil(t, built)
		assert.Contains(t, err.Error(), `build webhook "ops" renderer`)
	})
}

func TestWithAppData(t *testing.T) {
	t.Parallel()

	t.Run("clones custom data and attaches app data", func(t *testing.T) {
		t.Parallel()

		content := render.ContentTemplates{CustomData: map[string]string{"owner": "platform"}}
		got := withAppData(content, render.AppData{Version: "dev"})

		got.CustomData["owner"] = "changed"
		assert.Equal(t, "platform", content.CustomData["owner"])
		assert.Equal(t, "dev", got.App.Version)
	})
}
