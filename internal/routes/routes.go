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
	api *handler.API,
) http.Handler {
	mux := http.NewServeMux()

	mux.Handle(fmt.Sprintf("GET %s", checkInPath), api.CheckIn())
	mux.Handle(fmt.Sprintf("POST %s", checkInPath), api.CheckIn())
	mux.Handle("GET /status", api.Status())
	mux.Handle("GET /version", api.Version())
	mux.Handle("GET /metrics", api.Metrics())
	mux.Handle("GET /healthz", api.Healthz())
	mux.Handle("POST /healthz", api.Healthz())

	return httpprefix.MountUnderPrefix(mux, routePrefix)
}
