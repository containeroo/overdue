package flag

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containeroo/httpprefix"
	"github.com/containeroo/httputils"
	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/overdue/internal/notification/notifier"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/containeroo/tinyflags"
)

const (
	minAuthTokenLength = 32
	maxAuthTokenLength = 4096
)

// Config contains the fully parsed runtime configuration.
type Config struct {
	ListenAddr      string
	RoutePrefix     string
	CheckInName     string
	CheckInPath     string
	ExpectedEvery   time.Duration
	AlertingDelay   time.Duration
	StartActive     bool
	ResponseDetails bool
	AuthToken       string
	Debug           bool
	LogFormat       logging.LogFormat
	Notify          notifier.Config
}

// ParseArgs parses CLI arguments and OVERDUE-prefixed environment variables.
func ParseArgs(args []string, version string) (Config, error) {
	cfg := Config{}

	tf := tinyflags.NewFlagSet("overdue", tinyflags.ContinueOnError)
	tf.Version(version)
	tf.EnvPrefix("OVERDUE_")

	listenAddr := tf.TCPAddr("listen-address", &net.TCPAddr{IP: nil, Port: 8080}, "HTTP server listen address").
		Placeholder("ADDR:PORT").
		Validate(func(t *net.TCPAddr) error {
			if t.IP != nil && t.Port == 0 {
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

	tf.StringVar(&cfg.CheckInName, "check-in-name", "default", "Name of the check-in monitor used in notifications").
		Finalize(strings.TrimSpace).
		Placeholder("NAME").
		Validate(func(name string) error {
			if strings.TrimSpace(name) == "" {
				return errors.New("check-in-name must not be empty")
			}
			return nil
		}).
		Value()

	tf.StringVar(&cfg.CheckInPath, "check-in-path", "/check-in", "Single route used to receive check-ins").
		Finalize(func(path string) string {
			path = strings.TrimSpace(path)
			if path == "" {
				return "/check-in"
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			for len(path) > 1 && strings.HasSuffix(path, "/") {
				path = strings.TrimSuffix(path, "/")
			}
			return path
		}).
		FinalizeDefaultValue().
		Placeholder("PATH").
		Validate(func(path string) error {
			if path == "/" {
				return errors.New("check-in-path must be a non-root route")
			}
			return nil
		}).
		Value()

	tf.DurationVar(&cfg.ExpectedEvery, "expected-every", 0, "Maximum time between check-ins").
		Validate(func(value time.Duration) error {
			if value <= 0 {
				return errors.New("must be greater than zero")
			}
			return nil
		}).
		Required().
		Value()

	tf.DurationVar(&cfg.AlertingDelay, "alerting-delay", 0, "Extra time after the expected deadline before alerting").
		Validate(func(value time.Duration) error {
			if value <= 0 {
				return errors.New("must be greater than zero")
			}
			return nil
		}).
		Required().
		Value()

	tf.BoolVar(&cfg.StartActive, "start-active", false, "Activate the check-in monitor at startup instead of waiting for the first check-in").Value()
	tf.BoolVar(&cfg.ResponseDetails, "response-details", false, "Return detailed timing fields from check-in responses by default").Value()

	tf.StringVar(&cfg.AuthToken, "auth-token", "", "Optional bearer token required for check-ins").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast).
		Validate(func(token string) error {
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
		}).
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

	webhook := tf.DynamicGroup("webhook").Title("Webhooks")
	webhook.String("url", "", "Webhook URL").
		Validate(func(raw string) error {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return nil
			}

			u, err := url.Parse(raw)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return errors.New("must be a valid URL")
			}
			return nil
		}).
		Required().
		Placeholder("URL")
	tinyflags.DynamicEnum(
		webhook,
		"method",
		http.MethodPost,
		"HTTP method used for webhook requests",
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	).Placeholder("METHOD")
	webhook.Duration("timeout", 10*time.Second, "HTTP timeout").
		Validate(func(value time.Duration) error {
			if value <= 0 {
				return errors.New("must be greater than zero")
			}
			return nil
		}).
		Placeholder("DURATION")
	webhook.Bool("skip-insecure", false, "Skip TLS certificate verification")
	webhook.Bool("send-resolved", false, "Send a resolved webhook notification when check-ins resume after alerting")
	webhook.String("title-template", `[OVERDUE] Event Notification`, "Template for overdue webhook title")
	webhook.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Template for resolved webhook title")
	webhook.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Template for overdue webhook text")
	webhook.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Template for resolved webhook text")
	webhook.StringSlice("headers", nil, "HTTP headers in KEY=VALUE format").
		Validate(func(s string) error {
			_, err := httputils.ParseHeaders(s, false)
			return err
		}).
		Placeholder("KEY=VALUE")
	webhook.StringSlice("custom-data", nil, "Custom webhook template data in KEY=VALUE format").
		Validate(validateKeyValue).
		Placeholder("KEY=VALUE")
	webhook.String("template", "", "Path or builtin:<name> template for webhook JSON body").
		Required().
		Placeholder("PATH|builtin:NAME")
	tinyflags.DynamicEnum(
		webhook,
		"log-response",
		targets.LogResponseSummary,
		"Webhook response logging: summary, body, full, or none",
		targets.LogResponseSummary,
		targets.LogResponseBody,
		targets.LogResponseFull,
		targets.LogResponseNone,
	).Placeholder("MODE")
	webhook.Int("response-body-limit", 4096, "Maximum webhook response body bytes to read for logs and errors").
		Validate(func(i int) error {
			if i <= 0 {
				return errors.New("response-body-limit must be a positive integer")
			}
			return nil
		}).
		Placeholder("BYTES")

	email := tf.DynamicGroup("email").Title("Email")
	email.String("smtp-host", "", "SMTP host").
		Required().
		Placeholder("HOST")
	email.Int("smtp-port", 587, "SMTP port").
		Validate(func(i int) error {
			if i < 0 {
				return errors.New("smtp-port must be a positive integer")
			}
			return nil
		}).
		Placeholder("PORT")
	email.String("smtp-user", "", "SMTP username")
	email.String("smtp-pass", "", "SMTP password").
		Requires("smtp-user").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast)
	email.Bool("smtp-skip-insecure", false, "Skip SMTP TLS certificate verification")
	email.Bool("send-resolved", false, "Send a resolved email notification when check-ins resume after alerting")
	email.String("subject-template", "{{ .Title }}", "Template for overdue email subject")
	email.String("resolved-subject-template", "{{ .Title }}", "Template for resolved email subject")
	email.String("title-template", `[OVERDUE] Event Notification`, "Template for overdue email body title")
	email.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Template for resolved email body title")
	email.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Template for overdue email body text")
	email.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Template for resolved email body text")
	email.String("from", "", "Email sender address").
		Required().
		Placeholder("ADDR")
	email.StringSlice("to", []string{}, "Email recipient address").
		Required().
		Placeholder("ADDR")
	email.StringSlice("headers", nil, "Email headers in KEY=VALUE format").
		Validate(func(s string) error {
			_, err := httputils.ParseHeaders(s, false)
			return err
		}).
		Placeholder("KEY=VALUE")
	email.StringSlice("custom-data", nil, "Custom email template data in KEY=VALUE format").
		Validate(validateKeyValue).
		Placeholder("KEY=VALUE")
	email.String("template", "", "Path or builtin:<name> template for email body").
		Required().
		Placeholder("PATH|builtin:NAME")

	if err := tf.Parse(args); err != nil {
		return Config{}, err
	}

	notifyConfig, err := notifyConfigFromDynamicGroups(version, tf.DynamicGroups())
	if err != nil {
		return Config{}, err
	}

	cfg.ListenAddr = (*listenAddr).String()
	cfg.Notify = notifyConfig

	return cfg, nil
}

// validateKeyValue validates a key=value string.
func validateKeyValue(raw string) error {
	key, _, ok := strings.Cut(raw, "=")
	if !ok {
		return errors.New("must be in KEY=VALUE format")
	}

	if strings.TrimSpace(key) == "" {
		return errors.New("key must not be empty")
	}

	return nil
}
