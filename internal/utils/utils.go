package utils

import (
	"errors"
	"fmt"
	"strings"
)

// KeyValueMap creates a map from KEY=VALUE strings.
func KeyValueMap(groupName, id, field string, values []string) (map[string]string, error) {
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

// DefaultIfZero returns fallback when value is the zero value.
func DefaultIfZero[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}

// PtrIfNonZero returns nil when the value reports itself as zero.
func PtrIfNonZero[T interface{ IsZero() bool }](v T) *T {
	if v.IsZero() {
		return nil
	}

	return &v
}
