package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyConfigFromDynamicGroups(t *testing.T) {
	t.Parallel()

	t.Run("converts webhooks and emails in sorted instance order", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--webhook.zeta.url=https://example.test/zeta",
			"--webhook.zeta.template=zeta.tmpl",
			"--webhook.alpha.url=https://example.test/alpha",
			"--webhook.alpha.template=alpha.tmpl",
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.template=email.tmpl",
		})

		cfg, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.NoError(t, err)
		require.Len(t, cfg.Webhooks, 2)
		assert.Equal(t, "alpha", cfg.Webhooks[0].Name)
		assert.Equal(t, "zeta", cfg.Webhooks[1].Name)
		require.Len(t, cfg.Emails, 1)
		assert.Equal(t, "ops", cfg.Emails[0].Name)
	})

	t.Run("ignores unsupported empty dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")

		cfg, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.NoError(t, err)
		assert.Empty(t, cfg.Webhooks)
		assert.Empty(t, cfg.Emails)
	})

	t.Run("rejects unsupported populated dynamic group", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, nil)
		fs.DynamicGroup("pagerduty").String("routing-key", "", "routing key")
		require.NoError(t, fs.Parse([]string{"--pagerduty.ops.routing-key=secret"}))

		_, err := notifyConfigFromDynamicGroups("dev", fs.DynamicGroups())

		require.Error(t, err)
		assert.EqualError(t, err, `unsupported notification group "pagerduty"`)
	})
}

func TestSortedInstances(t *testing.T) {
	t.Parallel()

	fs := notifyTestFlagSet(t, []string{
		"--webhook.zeta.url=https://example.test/zeta",
		"--webhook.alpha.url=https://example.test/alpha",
		"--webhook.middle.url=https://example.test/middle",
	})

	assert.Equal(t, []string{"alpha", "middle", "zeta"}, sortedInstances(fs.DynamicGroup("webhook")))
}
