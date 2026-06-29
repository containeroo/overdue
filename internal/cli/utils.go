package cli

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/containeroo/httputils"
	"github.com/containeroo/overdue/internal/config"
)

// validateListenAddress validates the HTTP listen address.
func validateListenAddress(addr *net.TCPAddr) error {
	if addr == nil || addr.IP != nil && addr.Port == 0 {
		return errors.New("listen-address must not be empty")
	}
	return nil
}

// normalizePublicURL trims spaces and trailing slashes from the public URL.
func normalizePublicURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// validatePublicURL validates the externally reachable public URL.
func validatePublicURL(raw string) error {
	return validateAbsoluteURL(raw, true, "public-url must be a valid absolute URL")
}

// validateWebhookURL validates a webhook target URL.
func validateWebhookURL(raw string) error {
	return validateAbsoluteURL(raw, true, "must be a valid URL")
}

// validateAbsoluteURL validates that raw is an absolute URL.
//
// Empty values are accepted when allowEmpty is true. This is useful for flags
// that are later checked by tinyflags.Required().
func validateAbsoluteURL(raw string, allowEmpty bool, message string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" && allowEmpty {
		return nil
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New(message)
	}

	return nil
}

// validateCheckInName validates the configured check-in name.
func validateCheckInName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name must not be empty")
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

// validatePath validates the normalized check-in path.
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

	if len(token) < config.MinAuthTokenLength {
		return fmt.Errorf("auth-token must be at least %d characters", config.MinAuthTokenLength)
	}

	if len(token) > config.MaxAuthTokenLength {
		return fmt.Errorf("auth-token must be at most %d characters", config.MaxAuthTokenLength)
	}

	for i := 0; i < len(token); i++ {
		b := token[i]
		if b < 0x21 || b > 0x7e {
			return fmt.Errorf("auth-token must contain printable ASCII characters only; invalid byte 0x%02x at position %d", b, i)
		}
	}

	return nil
}

// validateHeader validates one HTTP or email header flag value.
func validateHeader(raw string) error {
	_, err := httputils.ParseHeaders(raw, false)
	return err
}

// intAtLeast returns a validator that requires values to be greater than or equal to minimum.
func intAtLeast(minimum int) func(int) error {
	return func(value int) error {
		if value < minimum {
			return fmt.Errorf("must be at least %d", minimum)
		}
		return nil
	}
}

// durationGreaterThanZero returns a validator that requires durations to be greater than zero.
func durationGreaterThanZero() func(time.Duration) error {
	return func(value time.Duration) error {
		if value <= 0 {
			return errors.New("must be greater than zero")
		}
		return nil
	}
}
