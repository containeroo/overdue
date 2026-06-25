package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"maps"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/target"
)

// Config contains one configured email notification target.
type Config struct {
	Name                    string
	Host                    string
	Port                    int
	User                    string
	Pass                    string
	SkipTLSVerify           bool
	SendResolved            bool
	From                    string
	To                      []string
	Headers                 map[string]string
	SubjectTemplate         string
	ResolvedSubjectTemplate string
	ContentTemplates        render.ContentTemplates
	Template                string
}

// Notifier sends check-in notifications via SMTP.
type Notifier struct {
	cfg      Config
	renderer Renderer
	logger   *slog.Logger
}

// New creates a new email notification target.
func New(cfg Config, renderer Renderer, logger *slog.Logger) Notifier {
	if logger == nil {
		panic("email logger must not be nil")
	}

	cfg.To = append([]string(nil), cfg.To...)
	cfg.Headers = maps.Clone(cfg.Headers)
	cfg.ContentTemplates = cfg.ContentTemplates.Clone()

	return Notifier{
		cfg:      cfg,
		renderer: renderer,
		logger:   logger,
	}
}

// Config returns a copy of the target configuration.
func (e Notifier) Config() Config {
	cfg := e.cfg
	cfg.To = append([]string(nil), cfg.To...)
	cfg.Headers = maps.Clone(cfg.Headers)
	cfg.ContentTemplates = cfg.ContentTemplates.Clone()
	return cfg
}

// Target returns public target metadata for status responses.
func (e Notifier) Target() target.Target {
	return target.Target{
		Type: "email",
		Name: e.cfg.Name,
	}
}

// Notify renders and sends an email target.
func (e Notifier) Notify(_ context.Context, event monitor.Event) error {
	if event.Resolved && !e.cfg.SendResolved {
		e.logger.Debug("resolved email skipped", "status", event.Status)
		return target.ErrSkipped
	}

	message, err := e.renderer.Render(event)
	if err != nil {
		return err
	}

	headers := []string{
		"From: " + e.cfg.From,
		"To: " + strings.Join(e.cfg.To, ", "),
		"Subject: " + message.Subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=utf-8",
	}
	headers = appendNotifierHeaders(headers, e.cfg.Headers)

	msg := strings.Join(append(headers,
		"",
		message.Body,
	), "\r\n")

	start := time.Now()
	if err := e.send(e.cfg.To, []byte(msg)); err != nil {
		duration := time.Since(start)
		e.logger.Error(
			"email delivery failed",
			"notificationStatus", event.Status,
			"smtpHost", e.cfg.Host,
			"smtpPort", e.cfg.Port,
			"recipients", len(e.cfg.To),
			"duration", duration.Round(time.Millisecond).String(),
			"error", err,
		)
		return err
	}

	duration := time.Since(start)
	e.logger.Info(
		"email delivered",
		"notificationStatus", event.Status,
		"smtpHost", e.cfg.Host,
		"smtpPort", e.cfg.Port,
		"recipients", len(e.cfg.To),
		"duration", duration.Round(time.Millisecond).String(),
	)

	return nil
}

// appendNotifierHeaders appends configured SMTP message headers in deterministic order.
func appendNotifierHeaders(lines []string, headers map[string]string) []string {
	if len(headers) == 0 {
		return lines
	}

	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		lines = append(lines, name+": "+headers[name])
	}
	return lines
}

// send delivers an SMTP message and applies the configured TLS behavior.
func (e Notifier) send(to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close() // nolint:errcheck

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(e.tlsConfig()); err != nil {
			return err
		}
	}

	if e.cfg.User != "" || e.cfg.Pass != "" {
		auth := smtp.PlainAuth("", e.cfg.User, e.cfg.Pass, e.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(e.cfg.From); err != nil {
		return err
	}

	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}

	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}

	return writer.Close()
}

// tlsConfig returns the SMTP TLS configuration.
func (e Notifier) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName:         e.cfg.Host,
		InsecureSkipVerify: e.cfg.SkipTLSVerify, // nolint:gosec // Explicitly controlled by email smtp-skip-insecure flag.
	}
}
