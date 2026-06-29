package flag

import (
	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/tinyflags"
)

// ParseArgs parses CLI arguments and OVERDUE-prefixed environment variables.
func ParseArgs(args []string, version string) (config.Config, error) {
	cfg := config.Config{}

	tf := tinyflags.NewFlagSet("overdue", tinyflags.ContinueOnError)
	tf.Version(version)
	tf.EnvPrefix("OVERDUE_")
	tf.HideEnvs()
	tf.Note("\nFlags can also be set through environment variables with the OVERDUE__ prefix. For example, --path becomes OVERDUE__PATH and --webhook.ops.url becomes OVERDUE__WEBHOOK_OPS_URL.")

	registerAppFlags(tf, &cfg)
	registerWebhookFlags(tf)
	registerEmailFlags(tf)

	if err := tf.Parse(args); err != nil {
		return config.Config{}, err
	}

	notifications, err := notificationsFromDynamicGroups(version, cfg.SiteRoot, cfg.CheckIn.Path, tf.DynamicGroups())
	if err != nil {
		return config.Config{}, err
	}
	cfg.Notifications = notifications
	cfg.OverriddenValues = tf.OverriddenValues()

	return cfg, nil
}
