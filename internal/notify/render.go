package notify

import (
	"fmt"
	"maps"
	"time"

	"github.com/containeroo/overdue/internal/config"
	"github.com/containeroo/overdue/internal/monitor"
)

const (
	appVar        = "App"
	customDataVar = "CustomData"
)

// AppData contains application links exposed to notification templates.
type AppData = config.AppData

// Data is the template context for check-in notifications.
type Data struct {
	IncidentID     string
	NotificationID string
	CheckInName    string
	LastCheckIn    time.Time
	ExpectedBy     time.Time
	OverdueSince   time.Time
	AlertingAt     time.Time
	Now            time.Time
	Phase          monitor.Phase
	Status         monitor.EventStatus
	Resolved       bool
	Title          string
	Text           string
	App            AppData
	Receiver       string
	Vars           map[string]any
	CustomData     map[string]string
}

// NewData builds template data from a monitor event and notifykit receiver vars.
func NewData(event monitor.Event, receiver string, vars map[string]any, title string) Data {
	return Data{
		IncidentID:     event.IncidentID,
		NotificationID: event.NotificationID,
		CheckInName:    event.CheckInName,
		LastCheckIn:    event.LastCheckIn,
		ExpectedBy:     event.ExpectedBy,
		OverdueSince:   event.OverdueSince,
		AlertingAt:     event.AlertingAt,
		Now:            event.Now,
		Phase:          event.Phase,
		Status:         event.Status,
		Resolved:       event.Resolved,
		Title:          title,
		Text:           text(event),
		App:            appData(vars),
		Receiver:       receiver,
		Vars:           publicVars(vars),
		CustomData:     customData(vars),
	}
}

// text returns the default plain text event summary.
func text(event monitor.Event) string {
	if event.Resolved || event.Status == monitor.StatusResolved {
		return fmt.Sprintf(`Check-in %q is resolved:`, event.CheckInName)
	}
	return fmt.Sprintf(`Check-in %q is overdue:`, event.CheckInName)
}

// varsFromConfig converts app and custom data into notifykit receiver variables.
func varsFromConfig(app AppData, custom map[string]string) map[string]any {
	vars := map[string]any{appVar: app}
	if len(custom) == 0 {
		return vars
	}

	customCopy := make(map[string]string, len(custom))
	for key, value := range custom {
		customCopy[key] = value
		vars[key] = value
	}
	vars[customDataVar] = customCopy
	return vars
}

// appData returns application template data from receiver variables.
func appData(vars map[string]any) AppData {
	if len(vars) == 0 {
		return AppData{}
	}
	app, _ := vars[appVar].(AppData)
	return app
}

// customData returns configured custom data from receiver variables.
func customData(vars map[string]any) map[string]string {
	if len(vars) == 0 {
		return nil
	}
	if custom, ok := vars[customDataVar].(map[string]string); ok {
		return cloneStringMap(custom)
	}

	custom := make(map[string]string, len(vars))
	for key, value := range vars {
		text, ok := value.(string)
		if ok {
			custom[key] = text
		}
	}
	if len(custom) == 0 {
		return nil
	}
	return custom
}

// publicVars returns receiver variables intended for templates.
func publicVars(vars map[string]any) map[string]any {
	if len(vars) == 0 {
		return nil
	}
	out := make(map[string]any, len(vars))
	for key, value := range vars {
		if key == appVar || key == customDataVar {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// cloneStringMap returns a defensive copy of values.
func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	maps.Copy(out, values)
	return out
}
