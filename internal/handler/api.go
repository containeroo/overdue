package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/service"
)

// CheckInService exposes the application behavior used by HTTP handler.
type CheckInService interface {
	RecordCheckIn(ctx context.Context, at time.Time) service.RecordCheckInResult
	Snapshot() service.SnapshotResult
}

// API bundles shared HTTP handler dependencies.
type API struct {
	authToken       string
	logger          *slog.Logger
	service         CheckInService
	metrics         *metrics.Registry
	nowFn           func() time.Time
	responseDetails bool
	version         string
	commit          string
}

// NewAPI builds an API container with shared handler dependencies.
func NewAPI(
	authToken string,
	service CheckInService,
	registry *metrics.Registry,
	responseDetails bool,
	version, commit string,
	logger *slog.Logger,
) *API {
	if service == nil {
		panic("check-in service must not be nil")
	}
	if registry == nil {
		panic("metrics registry must not be nil")
	}
	if logger == nil {
		panic("api logger must not be nil")
	}

	return &API{
		authToken:       authToken,
		logger:          logger,
		service:         service,
		metrics:         registry,
		nowFn:           time.Now,
		responseDetails: responseDetails,
		version:         version,
		commit:          commit,
	}
}

// SetNowFn replaces the API clock, primarily for tests.
func (a *API) SetNowFn(nowFn func() time.Time) {
	if nowFn == nil {
		panic("api clock must not be nil")
	}
	a.nowFn = nowFn
}
