package config

import (
	"fmt"

	"github.com/containeroo/httputils"
)

// HeaderMap creates a header map from configured header flag values.
func HeaderMap(groupName, id string, headers []string) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(headers))
	for _, raw := range headers {
		values, err := httputils.ParseHeaders(raw, false)
		if err != nil {
			return nil, fmt.Errorf("invalid \"--%s.%s.headers\": %w", groupName, id, err)
		}

		for name, value := range values {
			if _, exists := parsed[name]; exists {
				return nil, fmt.Errorf("invalid \"--%s.%s.headers\": duplicate header key found: %s", groupName, id, name)
			}
			parsed[name] = value
		}
	}

	if len(parsed) == 0 {
		return nil, nil
	}

	return parsed, nil
}
