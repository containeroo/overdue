package handler

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// authorized reports whether the request satisfies the configured bearer token policy.
func (a *API) authorized(r *http.Request) bool {
	if a.authToken == "" {
		return true
	}

	token := bearerToken(r.Header.Get("Authorization"))
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.authToken)) == 1
}

// bearerToken extracts a bearer token from an Authorization header.
func bearerToken(header string) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
