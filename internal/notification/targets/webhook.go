package targets

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/delivery"
	"github.com/containeroo/overdue/internal/notification/render"
)

// LogResponse controls how much of a webhook response is logged on success.
type LogResponse string

const (
	// LogResponseSummary logs only status, status code, duration, and truncation state.
	LogResponseSummary LogResponse = "summary"
	// LogResponseBody logs status fields and response body, but not response headers.
	LogResponseBody LogResponse = "body"
	// LogResponseFull logs status fields, response body, and response headers.
	LogResponseFull LogResponse = "full"
	// LogResponseNone suppresses successful webhook response logs.
	LogResponseNone LogResponse = "none"
)

// WebhookConfig contains one configured webhook notification target.
type WebhookConfig struct {
	Name              string
	URL               string
	Method            string
	Headers           map[string]string
	Timeout           time.Duration
	SkipInsecure      bool
	SendResolved      bool
	ContentTemplates  render.ContentTemplates
	Template          string
	LogResponse       LogResponse
	ResponseBodyLimit int
}

// Webhook sends check-in notifications to a generic JSON webhook endpoint.
type Webhook struct {
	cfg      WebhookConfig
	renderer WebhookRenderer
	client   *http.Client
	logger   *slog.Logger
}

// NewWebhook creates a generic webhook notifier with the configured HTTP client.
func NewWebhook(cfg WebhookConfig, renderer WebhookRenderer, logger *slog.Logger) Webhook {
	if logger == nil {
		panic("webhook logger must not be nil")
	}
	if cfg.Timeout <= 0 {
		panic("webhook timeout must be greater than zero")
	}
	if cfg.ResponseBodyLimit < 0 {
		panic("webhook response body limit must not be negative")
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	if cfg.LogResponse == "" {
		cfg.LogResponse = LogResponseSummary
	}
	cfg.Headers = maps.Clone(cfg.Headers)
	cfg.ContentTemplates = cfg.ContentTemplates.Clone()

	return Webhook{
		cfg:      cfg,
		renderer: renderer,
		client:   webhookClient(cfg.Timeout, cfg.SkipInsecure),
		logger:   logger,
	}
}

// Config returns a copy of the target configuration.
func (w Webhook) Config() WebhookConfig {
	cfg := w.cfg
	cfg.Headers = maps.Clone(cfg.Headers)
	cfg.ContentTemplates = cfg.ContentTemplates.Clone()
	return cfg
}

// Client returns the configured HTTP client.
func (w Webhook) Client() *http.Client {
	return w.client
}

// NotificationTarget returns public target metadata for status responses.
func (w Webhook) NotificationTarget() delivery.Target {
	return delivery.Target{
		Type: "webhook",
		Name: w.cfg.Name,
	}
}

// Notify renders and posts a webhook delivery.
func (w Webhook) Notify(ctx context.Context, event monitor.Event) error {
	if event.Resolved && !w.cfg.SendResolved {
		w.logger.Debug("resolved webhook skipped", "status", event.Status)
		return delivery.ErrSkipped
	}

	event, body, err := w.renderer.RenderBody(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, w.cfg.Method, w.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	for name, value := range w.cfg.Headers {
		req.Header.Set(name, value)
	}

	start := time.Now()
	resp, err := w.client.Do(req)
	duration := time.Since(start)
	if err != nil {
		w.logger.Error(
			"webhook request failed",
			"status", event.Status,
			"duration", duration.Round(time.Millisecond).String(),
			"error", err,
		)
		return err
	}
	defer resp.Body.Close() // nolint:errcheck

	responseBody, truncated, err := readWebhookResponseBody(resp.Body, w.cfg.ResponseBodyLimit)
	if err != nil {
		w.logger.Error(
			"webhook response read failed",
			"status", resp.Status,
			"notificationStatus", event.Status,
			"statusCode", resp.StatusCode,
			"duration", duration.Round(time.Millisecond).String(),
			"error", err,
		)
		return fmt.Errorf("%s read response body: %w", w.label(), err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := w.responseError(resp, responseBody, truncated)
		fields := w.responseLogFields(resp, responseBody, truncated, duration, LogResponseBody)
		fields = append(fields, "notificationStatus", event.Status, "error", err)

		w.logger.Error("webhook delivery failed", fields...)
		return err
	}

	w.logSuccessfulResponse(event, resp, responseBody, truncated, duration)
	return nil
}

// logSuccessfulResponse logs a successful webhook response according to the configured policy.
func (w Webhook) logSuccessfulResponse(
	event monitor.Event,
	resp *http.Response,
	body string,
	truncated bool,
	duration time.Duration,
) {
	fields := []any{"notificationStatus", event.Status}

	switch w.cfg.LogResponse {
	case LogResponseNone:
		return
	case LogResponseBody, LogResponseFull:
		fields = append(fields, w.responseLogFields(resp, body, truncated, duration, w.cfg.LogResponse)...)
	default:
		fields = append(fields, w.responseLogFields(resp, body, truncated, duration, LogResponseSummary)...)
	}

	w.logger.Info("webhook delivered", fields...)
}

// responseLogFields builds flattened structured response fields for logging.
func (w Webhook) responseLogFields(
	resp *http.Response,
	body string,
	truncated bool,
	duration time.Duration,
	mode LogResponse,
) []any {
	fields := []any{
		"status", resp.Status,
		"statusCode", resp.StatusCode,
		"duration", duration.Round(time.Millisecond).String(),
		"bodyTruncated", truncated,
	}

	switch mode {
	case LogResponseBody:
		fields = append(fields, "responseBody", webhookResponseBodyValue(body))
	case LogResponseFull:
		fields = append(
			fields,
			"responseBody", webhookResponseBodyValue(body),
			"responseHeaders", cloneResponseHeaders(resp.Header),
		)
	}

	return fields
}

// responseError converts non-2xx HTTP responses into errors.
func (w Webhook) responseError(resp *http.Response, body string, truncated bool) error {
	bodyForError := body
	if truncated {
		bodyForError += "...(truncated)"
	}

	if bodyForError == "" {
		return fmt.Errorf("%s returned %s", w.label(), resp.Status)
	}
	return fmt.Errorf("%s returned %s: %s", w.label(), resp.Status, bodyForError)
}

// readWebhookResponseBody reads and caps webhook response bodies for logs and errors.
func readWebhookResponseBody(body io.Reader, limit int) (responseBody string, truncated bool, err error) {
	if limit < 0 {
		return "", false, fmt.Errorf("response body limit must not be negative")
	}

	data, err := io.ReadAll(io.LimitReader(body, int64(limit)+1))
	if err != nil {
		return "", false, err
	}

	truncated = len(data) > limit
	if truncated {
		data = data[:limit]
	}

	return strings.TrimSpace(string(data)), truncated, nil
}

// webhookResponseBodyValue returns JSON responses as structured values and all other responses as strings.
func webhookResponseBodyValue(body string) any {
	if body == "" {
		return nil
	}

	var value any
	if err := json.Unmarshal([]byte(body), &value); err == nil {
		return value
	}

	return body
}

// cloneResponseHeaders copies response headers for structured logging.
func cloneResponseHeaders(headers http.Header) (clone map[string][]string) {
	if len(headers) == 0 {
		return nil
	}

	clone = make(map[string][]string, len(headers))
	for name, values := range headers {
		clone[name] = append([]string(nil), values...)
	}
	return clone
}

// webhookClient builds the HTTP client used for webhook delivery.
func webhookClient(timeout time.Duration, skipInsecure bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment

	if skipInsecure {
		tlsConfig := &tls.Config{}
		if transport.TLSClientConfig != nil {
			tlsConfig = transport.TLSClientConfig.Clone()
		}
		tlsConfig.InsecureSkipVerify = true // nolint:gosec // Explicitly controlled by webhook skip-insecure flags.
		transport.TLSClientConfig = tlsConfig
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// label returns the human-readable target label used in errors.
func (w Webhook) label() string {
	if w.cfg.Name == "" {
		return "webhook"
	}
	return fmt.Sprintf("webhook %q", w.cfg.Name)
}
