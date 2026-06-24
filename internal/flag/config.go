package flag

import (
	"net"
	"time"

	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/overdue/internal/notification/notifier"
)

const (
	minAuthTokenLength = 32
	maxAuthTokenLength = 4096
)

// Config contains the fully parsed runtime configuration.
type Config struct {
	ListenAddr      *net.TCPAddr
	RoutePrefix     string
	PublicURL       string
	ResponseDetails bool
	AuthToken       string
	Debug           bool
	LogFormat       logging.LogFormat
	CheckIn         CheckInConfig
	Notify          notifier.Config
}

// CheckInConfig contains the parsed check-in schedule and endpoint configuration.
type CheckInConfig struct {
	Name          string
	Path          string
	ExpectedEvery time.Duration
	AlertingDelay time.Duration
	StartActive   bool
}
