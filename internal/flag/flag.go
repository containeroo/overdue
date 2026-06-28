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

	if err := config.FromFlags(&cfg, version, tf.DynamicGroups()); err != nil {
		return config.Config{}, err
	}
	cfg.Overridden = tf.OverriddenValues()

	return cfg, nil
}
