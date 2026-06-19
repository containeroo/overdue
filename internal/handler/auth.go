package handler

import (
	"net/http"
	"strings"
)

// authorized reports whether the request satisfies the configured bearer token policy.
func (a *API) authorized(r *http.Request) bool {
	return a.authToken == "" || bearerToken(r.Header.Get("Authorization")) == a.authToken
}

// bearerToken extracts a bearer token from an Authorization header.
func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
