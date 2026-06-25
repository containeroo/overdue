package config

import (
	"net"
	"time"

	"github.com/containeroo/notifykit/targets/webhook"
	"github.com/containeroo/overdue/internal/logging"
)

const (
	// MinAuthTokenLength is the minimum accepted bearer token length.
	MinAuthTokenLength = 32
	// MaxAuthTokenLength is the maximum accepted bearer token length.
	MaxAuthTokenLength = 4096
)

// Config contains the fully normalized runtime configuration.
type Config struct {
	ListenAddr      *net.TCPAddr
	RoutePrefix     string
	SiteRoot        string
	ResponseDetails bool
	AuthToken       string
	Debug           bool
	LogFormat       logging.LogFormat
	CheckIn         CheckInConfig
	Notifications   Notifications
}

// CheckInConfig contains the check-in schedule and endpoint configuration.
type CheckInConfig struct {
	Name          string
	Path          string
	ExpectedEvery time.Duration
	AlertingDelay time.Duration
	StartActive   bool
}

// Notifications contains all configured notification targets.
type Notifications struct {
	App      AppData
	Webhooks []WebhookConfig
	Emails   []EmailConfig
}

// AppData contains application links exposed to notification templates.
type AppData struct {
	Version    string
	SiteRoot   string
	CheckInURL string
	StatusURL  string
}

// LogResponse controls how much of a webhook response is logged on success.
type LogResponse = webhook.LogResponse

// WebhookConfig contains one configured webhook notification target.
type WebhookConfig struct {
	Name              string
	URL               string
	Method            string
	Headers           map[string]string
	Timeout           time.Duration
	SkipInsecure      bool
	SendResolved      bool
	SubjectTemplate   string
	Template          string
	CustomData        map[string]string
	LogResponse       LogResponse
	ResponseBodyLimit int
}

// EmailConfig contains one configured email notification target.
type EmailConfig struct {
	Name            string
	Host            string
	Port            int
	User            string
	Pass            string
	SkipTLSVerify   bool
	SendResolved    bool
	From            string
	To              []string
	Headers         map[string]string
	SubjectTemplate string
	Template        string
	CustomData      map[string]string
}

const (
	// LogResponseSummary logs only status, status code, duration, and truncation state.
	LogResponseSummary = webhook.LogResponseSummary
	// LogResponseBody logs status fields and response body, but not response headers.
	LogResponseBody = webhook.LogResponseBody
	// LogResponseFull logs status fields, response body, and response headers.
	LogResponseFull = webhook.LogResponseFull
	// LogResponseNone suppresses successful webhook response logs.
	LogResponseNone = webhook.LogResponseNone
)
