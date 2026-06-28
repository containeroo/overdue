package routes

import (
	"fmt"
	"net/http"

	"github.com/containeroo/httpprefix"
	"github.com/containeroo/overdue/internal/handler"
)

// NewRouter builds the service router.
func NewRouter(
	checkInPath, routePrefix string,
	allowGETCheckIn bool,
	api *handler.API,
) http.Handler {
	mux := http.NewServeMux()

	if allowGETCheckIn {
		mux.Handle(fmt.Sprintf("GET %s", checkInPath), api.CheckIn())
	}
	mux.Handle(fmt.Sprintf("POST %s", checkInPath), api.CheckIn())
	mux.Handle("GET /status", api.Status())
	mux.Handle("GET /version", api.Version())
	mux.Handle("GET /metrics", api.Metrics())
	mux.Handle("GET /healthz", api.Healthz())
	mux.Handle("POST /healthz", api.Healthz())
	mux.Handle("GET /readyz", api.Readyz())
	mux.Handle("POST /readyz", api.Readyz())

	return httpprefix.MountUnderPrefix(mux, routePrefix)
}
