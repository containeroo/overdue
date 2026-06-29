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
