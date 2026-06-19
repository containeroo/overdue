package handler

import (
	"net"
	"net/http"
	"strings"
)

// RequestMetadata describes the request origin used for diagnostic logs.
//
// Remote is the direct TCP peer that connected to this service. ClientIP is a
// best-effort value from proxy headers first, then from Remote. Do not use it
// for auth decisions unless your proxy chain sanitizes these headers.
type RequestMetadata struct {
	Remote          string
	ClientIP        string
	Method          string
	Path            string
	Host            string
	Proto           string
	UserAgent       string
	RequestID       string
	XForwardedFor   string
	XForwardedProto string
	XForwardedHost  string
	XRealIP         string
	Forwarded       string
}

// requestMetadata returns diagnostic request-origin metadata for logs.
func requestMetadata(r *http.Request) RequestMetadata {
	if r == nil {
		return RequestMetadata{}
	}

	forwardedFor := headerValue(r, "X-Forwarded-For")
	forwarded := headerValue(r, "Forwarded")
	xRealIP := headerValue(r, "X-Real-IP")

	return RequestMetadata{
		Remote:          strings.TrimSpace(r.RemoteAddr),
		ClientIP:        bestEffortClientIP(r.RemoteAddr, forwardedFor, forwarded, xRealIP),
		Method:          strings.TrimSpace(r.Method),
		Path:            requestPath(r),
		Host:            strings.TrimSpace(r.Host),
		Proto:           strings.TrimSpace(r.Proto),
		UserAgent:       strings.TrimSpace(r.UserAgent()),
		RequestID:       firstNonEmpty(headerValue(r, "X-Request-ID"), headerValue(r, "X-Correlation-ID"), headerValue(r, "Traceparent")),
		XForwardedFor:   forwardedFor,
		XForwardedProto: headerValue(r, "X-Forwarded-Proto"),
		XForwardedHost:  headerValue(r, "X-Forwarded-Host"),
		XRealIP:         xRealIP,
		Forwarded:       forwarded,
	}
}

// headerValue returns the trimmed value of a request header.
func headerValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.Header.Get(name))
}

// LogFields returns structured request-origin fields for logs.
func (m RequestMetadata) LogFields() []any {
	fields := make([]any, 0, 26)
	fields = appendNonEmptyLogField(fields, "remote", m.Remote)
	fields = appendNonEmptyLogField(fields, "clientIP", m.ClientIP)
	fields = appendNonEmptyLogField(fields, "method", m.Method)
	fields = appendNonEmptyLogField(fields, "path", m.Path)
	fields = appendNonEmptyLogField(fields, "host", m.Host)
	fields = appendNonEmptyLogField(fields, "proto", m.Proto)
	fields = appendNonEmptyLogField(fields, "userAgent", m.UserAgent)
	fields = appendNonEmptyLogField(fields, "requestID", m.RequestID)
	fields = appendNonEmptyLogField(fields, "xForwardedFor", m.XForwardedFor)
	fields = appendNonEmptyLogField(fields, "xForwardedProto", m.XForwardedProto)
	fields = appendNonEmptyLogField(fields, "xForwardedHost", m.XForwardedHost)
	fields = appendNonEmptyLogField(fields, "xRealIP", m.XRealIP)
	fields = appendNonEmptyLogField(fields, "forwarded", m.Forwarded)

	return fields
}

// wantsDetails reports whether this request should receive a detailed response.
func (a *API) wantsDetails(r *http.Request) bool {
	return a.responseDetails || wantsDetails(r)
}

// wantsDetails reports whether the response should include timing details.
func wantsDetails(r *http.Request) bool {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("details")))
	return value == "true" || value == "1" || value == "yes"
}

// requestPath returns the escaped request path without query parameters.
func requestPath(r *http.Request) string {
	if r.URL == nil {
		return ""
	}
	return r.URL.EscapedPath()
}

// bestEffortClientIP returns a diagnostic client IP from common proxy headers.
func bestEffortClientIP(remoteAddr, forwardedFor, forwarded, xRealIP string) string {
	if ip := firstForwardedFor(forwardedFor); ip != "" {
		return ip
	}
	if ip := forwardedForParam(forwarded); ip != "" {
		return ip
	}
	if ip := hostOnly(xRealIP); ip != "" {
		return ip
	}
	return hostOnly(remoteAddr)
}

// firstForwardedFor returns the first address in X-Forwarded-For.
func firstForwardedFor(value string) string {
	first, _, _ := strings.Cut(value, ",")
	return hostOnly(first)
}

// forwardedForParam extracts the first for= value from the standardized Forwarded header.
func forwardedForParam(value string) string {
	first, _, _ := strings.Cut(value, ",")

	for part := range strings.SplitSeq(first, ";") {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "for") {
			continue
		}

		raw = strings.Trim(strings.TrimSpace(raw), `"`)
		if strings.EqualFold(raw, "unknown") {
			return ""
		}
		return hostOnly(raw)
	}

	return ""
}

// hostOnly returns the host part of an address when possible.
func hostOnly(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}

	value = strings.Trim(value, "[]")
	if ip := net.ParseIP(value); ip != nil {
		return ip.String()
	}

	return value
}

// firstNonEmpty returns the first non-blank value.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// appendNonEmptyLogField appends key/value when value is not blank.
func appendNonEmptyLogField(fields []any, key, value string) []any {
	value = strings.TrimSpace(value)
	if value == "" {
		return fields
	}
	return append(fields, key, value)
}
