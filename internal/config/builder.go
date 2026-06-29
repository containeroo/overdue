package config

import (
	"net/url"
	"strings"
)

// NewAppData builds template app data from normalized application settings.
func NewAppData(version, siteRoot, routePrefix, checkInPath string) AppData {
	app := AppData{Version: version}
	if strings.TrimSpace(siteRoot) == "" {
		return app
	}

	app.SiteRoot = publicAppRoot(siteRoot, routePrefix)
	app.CheckInURL = joinURLPath(app.SiteRoot, checkInPath)
	app.StatusURL = joinURLPath(app.SiteRoot, "/status")
	return app
}

// publicAppRoot returns the externally reachable root for mounted service routes.
func publicAppRoot(siteRoot, routePrefix string) string {
	siteRoot = strings.TrimRight(strings.TrimSpace(siteRoot), "/")
	routePrefix = strings.TrimSpace(routePrefix)
	if routePrefix == "" || routePrefix == "/" || hasURLPathSuffix(siteRoot, routePrefix) {
		return siteRoot
	}
	return joinURLPath(siteRoot, routePrefix)
}

// hasURLPathSuffix reports whether rawURL already ends in the given normalized path.
func hasURLPathSuffix(rawURL, path string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), strings.TrimRight(path, "/"))
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
