package config

import (
	"net"
	"time"

	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/webhook"
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
	PublicURL       string
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
	App      render.AppData
	Webhooks []webhook.Config
	Emails   []email.Config
}
