package flag

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/containeroo/httpprefix"
	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/tinyflags"
)

// registerAppFlags registers core application flags and binds them to cfg.
func registerAppFlags(tf *tinyflags.FlagSet, cfg *Config) {
	tf.TCPAddrVar(&cfg.ListenAddr, "listen-address", &net.TCPAddr{IP: nil, Port: 8080}, "HTTP server listen address").
		Placeholder("ADDR:PORT").
		Validate(func(addr *net.TCPAddr) error {
			if addr.IP != nil && addr.Port == 0 {
				return errors.New("listen-address must not be empty")
			}
			return nil
		}).
		Value()

	tf.StringVar(&cfg.RoutePrefix, "route-prefix", "", "Path prefix to mount the service. Empty = root.").
		Finalize(httpprefix.NormalizeRoutePrefix).
		FinalizeDefaultValue().
		Placeholder("PATH").
		Value()

	tf.StringVar(&cfg.PublicURL, "public-url", "", "Externally reachable base URL used in notification templates").
		Finalize(normalizePublicURL).
		FinalizeDefaultValue().
		Placeholder("URL").
		Validate(validatePublicURL).
		Value()

	tf.StringVar(&cfg.CheckIn.Name, "name", "default", "Name of the check-in monitor used in notifications").
		Finalize(strings.TrimSpace).
		Placeholder("NAME").
		Validate(func(name string) error {
			if strings.TrimSpace(name) == "" {
				return errors.New("name must not be empty")
			}
			return nil
		}).
		Value()

	tf.StringVar(&cfg.CheckIn.Path, "path", "/checkin", "Single route used to receive check-ins").
		Finalize(normalizePath).
		FinalizeDefaultValue().
		Placeholder("PATH").
		Validate(validatePath).
		Value()

	tf.DurationVar(&cfg.CheckIn.ExpectedEvery, "expected-every", 0, "Maximum time between check-ins").
		Validate(func(value time.Duration) error {
			if value <= 0 {
				return errors.New("must be greater than zero")
			}
			return nil
		}).
		Required().
		Value()

	tf.DurationVar(&cfg.CheckIn.AlertingDelay, "alerting-delay", 0, "Extra time after the expected deadline before alerting").
		Validate(func(value time.Duration) error {
			if value <= 0 {
				return errors.New("must be greater than zero")
			}
			return nil
		}).
		Required().
		Value()

	tf.BoolVar(&cfg.CheckIn.StartActive, "start-active", false, "Activate the check-in monitor at startup instead of waiting for the first check-in").Value()
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

// normalizePublicURL trims spaces and trailing slashes from the public URL.
func normalizePublicURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// validatePublicURL validates the externally reachable public URL.
func validatePublicURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("public-url must be a valid absolute URL")
	}

	return nil
}

// normalizePath normalizes the route used to receive check-ins.
func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/checkin"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// validatePath validates the normalized path.
func validatePath(path string) error {
	if path == "/" {
		return errors.New("path must be a non-root route")
	}
	return nil
}

// validateAuthToken validates the optional bearer token policy.
func validateAuthToken(token string) error {
	if token == "" {
		return nil
	}

	if len(token) < minAuthTokenLength {
		return fmt.Errorf("auth-token must be at least %d characters", minAuthTokenLength)
	}

	if len(token) > maxAuthTokenLength {
		return fmt.Errorf("auth-token must be at most %d characters", maxAuthTokenLength)
	}

	for i := 0; i < len(token); i++ {
		b := token[i]
		if b < 0x21 || b > 0x7e {
			return fmt.Errorf("auth-token must contain printable ASCII characters only; invalid byte 0x%02x at position %d", b, i)
		}
	}

	return nil
}
