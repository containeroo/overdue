package target

import (
	"errors"
	"strings"

	"github.com/containeroo/overdue/internal/monitor"
)

// NotificationKey returns the stable key used to track one notification delivery.
func NotificationKey(event monitor.Event) (string, error) {
	key := strings.TrimSpace(event.NotificationID)
	if key == "" {
		return "", errors.New("notification event is missing notification id")
	}
	return key, nil
}
