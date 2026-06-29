package cli

import (
	"net"
	"strings"

	"github.com/containeroo/httpprefix"
	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/tinyflags"
)

// registerAppFlags registers core application flags and binds them to cfg.
func registerAppFlags(tf *tinyflags.FlagSet, cfg *config.Config) {
	tf.TCPAddrVar(&cfg.ListenAddr, "listen-address", &net.TCPAddr{IP: nil, Port: 8080}, "HTTP server listen address").
		Placeholder("ADDR:PORT").
		Validate(validateListenAddress).
		Value()

	tf.StringVar(&cfg.RoutePrefix, "route-prefix", "", "Path prefix to mount the service. Empty = root.").
		Finalize(httpprefix.NormalizeRoutePrefix).
		FinalizeDefaultValue().
		Placeholder("PATH").
		Value()

	tf.StringVar(&cfg.SiteRoot, "public-url", "", "Externally reachable base URL used in notification templates").
		Finalize(normalizePublicURL).
		FinalizeDefaultValue().
		Placeholder("URL").
		Validate(validatePublicURL).
		Value()

	tf.StringVar(&cfg.CheckIn.Name, "name", "default", "Name of the check-in monitor used in notifications").
		Finalize(strings.TrimSpace).
		Placeholder("NAME").
		Validate(validateCheckInName).
		Value()

	tf.StringVar(&cfg.CheckIn.Path, "path", "/checkin", "Single route used to receive check-ins").
		Finalize(normalizePath).
		FinalizeDefaultValue().
		Placeholder("PATH").
		Validate(validatePath).
		Value()

	tf.DurationVar(&cfg.CheckIn.ExpectedEvery, "expected-every", 0, "Maximum time between check-ins").
		Placeholder("DURATION").
		Required().
		HideDefault().
		Validate(durationGreaterThanZero()).
		Value()

	tf.DurationVar(&cfg.CheckIn.AlertingDelay, "alerting-delay", 0, "Extra time after the expected deadline before alerting").
		Placeholder("DURATION").
		Required().
		HideDefault().
		Validate(durationGreaterThanZero()).
		Value()

	tf.BoolVar(&cfg.CheckIn.StartActive, "start-active", false, "Activate the check-in monitor at startup instead of waiting for the first check-in").Value()
	tf.BoolVar(&cfg.CheckIn.AllowGET, "allow-get-checkin", false, "Also accept GET requests on the check-in route").Value()
	tf.BoolVar(&cfg.ResponseDetails, "response-details", false, "Return detailed timing fields from check-in responses by default").Value()

	tf.StringVar(&cfg.AuthToken, "auth-token", "", "Optional bearer token required for check-ins").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast).
		Validate(validateAuthToken).
		Value()

	tf.BoolVar(&cfg.Debug, "debug", false, "Enable debug logging").Value()

	tinyflags.EnumVar(
		tf,
		&cfg.LogFormat,
		"log-format",
		logging.LogFormatJSON,
		"Log format",
		logging.LogFormatJSON,
		logging.LogFormatText,
	).Value()
}
