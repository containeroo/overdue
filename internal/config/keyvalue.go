package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/containeroo/httputils"
)

// ValidateKeyValue validates a KEY=VALUE option.
func ValidateKeyValue(raw string) error {
	key, _, ok := strings.Cut(raw, "=")
	if !ok {
		return errors.New("must be in KEY=VALUE format")
	}

	if strings.TrimSpace(key) == "" {
		return errors.New("key must not be empty")
	}

	return nil
}

// headerMap creates a header map from KEY=VALUE strings.
func headerMap(groupName, id string, headers []string) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(headers))
	for _, header := range headers {
		values, err := httputils.ParseHeaders(header, false)
		if err != nil {
			return nil, fmt.Errorf("invalid \"--%s.%s.headers\": %w", groupName, id, err)
		}

		for name, value := range values {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("invalid \"--%s.%s.headers\": header name must not be empty", groupName, id)
			}
			parsed[name] = value
		}
	}

	return parsed, nil
}

// keyValueMap creates a map from KEY=VALUE strings.
func keyValueMap(groupName, id, field string, values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(values))
	for _, raw := range values {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("invalid \"--%s.%s.%s\": must be in KEY=VALUE format", groupName, id, field)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid \"--%s.%s.%s\": key must not be empty", groupName, id, field)
		}
		parsed[key] = value
	}

	return parsed, nil
}
