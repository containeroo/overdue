package config

import (
	"strings"
)

const (
	defaultSubjectTemplate = `{{ if .Resolved }}[RESOLVED]{{ else }}[OVERDUE]{{ end }} Event Notification`
)

// DefaultSubjectTemplate returns the default notification subject template.
func DefaultSubjectTemplate() string {
	return defaultSubjectTemplate
}

// NewAppData builds template app data from normalized application settings.
func NewAppData(version, siteRoot, checkInPath string) AppData {
	app := AppData{Version: version}
	if strings.TrimSpace(siteRoot) == "" {
		return app
	}

	app.SiteRoot = siteRoot
	app.CheckInURL = joinURLPath(siteRoot, checkInPath)
	app.StatusURL = joinURLPath(siteRoot, "/status")
	return app
}

// joinURLPath appends path to base using the simple URL behavior Overdue needs.
func joinURLPath(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
