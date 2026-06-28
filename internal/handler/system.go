package handler

import "net/http"

// Healthz returns a lightweight liveness endpoint.
func (a *API) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		a.respondText(w, http.StatusOK, "ok")
	}
}

// Readyz returns a lightweight readiness endpoint.
func (a *API) Readyz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		a.respondText(w, http.StatusOK, "ok")
	}
}

// Metrics returns the Prometheus metrics endpoint handler.
func (a *API) Metrics() http.Handler {
	return a.metrics.Metrics()
}

// Version returns the running build version and commit.
func (a *API) Version() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.logger.Debug("version requested", requestMetadata(r).LogFields()...)
		a.respondJSON(w, http.StatusOK, versionResponse{Version: a.version, Commit: a.commit})
	}
}
