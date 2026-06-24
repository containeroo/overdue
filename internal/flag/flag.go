package flag

import (
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/tinyflags"
)

// ParseArgs parses CLI arguments and OVERDUE-prefixed environment variables.
func ParseArgs(args []string, version string) (Config, error) {
	cfg := Config{}

	tf := tinyflags.NewFlagSet("overdue", tinyflags.ContinueOnError)
	tf.Version(version)
	tf.EnvPrefix("OVERDUE_")

	registerAppFlags(tf, &cfg)
	registerWebhookFlags(tf)
	registerEmailFlags(tf)

	if err := tf.Parse(args); err != nil {
		return Config{}, err
	}

	notifyConfig, err := notifyConfigFromDynamicGroups(version, tf.DynamicGroups())
	if err != nil {
		return Config{}, err
	}
	notifyConfig.App = render.NewAppData(version, cfg.PublicURL, cfg.CheckIn.Path)

	cfg.Notify = notifyConfig

	return cfg, nil
}
