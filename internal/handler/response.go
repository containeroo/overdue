package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/containeroo/overdue/internal/utils"
)

// statusResponse is the standard success payload.
type statusResponse struct {
	Status string `json:"status"`
}

// errorResponse is the standard error payload.
type errorResponse struct {
	Error string `json:"error"`
}

// versionResponse is the build information payload.
type versionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// checkInResponseStatus describes the command result returned by accepted check-in responses.
type checkInResponseStatus string

const (
	// checkInResponseStatusOK means the check-in request was accepted and recorded.
	checkInResponseStatusOK checkInResponseStatus = "ok"
)

// checkInResponse is the compact JSON representation of an accepted check-in.
type checkInResponse struct {
	Status checkInResponseStatus `json:"status"`
}

// snapshotResponse is the compact JSON representation of monitor state.
type snapshotResponse struct {
	LastCheckIn  *time.Time    `json:"lastCheckIn,omitempty"`
	ExpectedBy   *time.Time    `json:"expectedBy,omitempty"`
	OverdueSince *time.Time    `json:"overdueSince,omitempty"`
	AlertingAt   *time.Time    `json:"alertingAt,omitempty"`
	Phase        monitor.Phase `json:"phase"`
}

// checkInDetailsResponse is the detailed timing JSON payload for status and check-in responses.
type checkInDetailsResponse struct {
	Status        checkInResponseStatus       `json:"status,omitempty"`
	CheckInName   string                      `json:"checkInName"`
	Phase         monitor.Phase               `json:"phase"`
	LastCheckIn   *time.Time                  `json:"lastCheckIn,omitempty"`
	ExpectedBy    *time.Time                  `json:"expectedBy,omitempty"`
	ExpectedEvery string                      `json:"expectedEvery,omitempty"`
	OverdueSince  *time.Time                  `json:"overdueSince,omitempty"`
	OverdueFor    string                      `json:"overdueFor,omitempty"`
	AlertingAt    *time.Time                  `json:"alertingAt,omitempty"`
	AlertingDelay string                      `json:"alertingDelay,omitempty"`
	AlertingAfter string                      `json:"alertingAfter,omitempty"`
	AlertingFor   string                      `json:"alertingFor,omitempty"`
	Notifications *notificationStatusResponse `json:"notifications,omitempty"`
}

// notificationStatusResponse is the aggregate notification delivery status.
type notificationStatusResponse struct {
	Status    target.DeliveryStatus        `json:"status"`
	Total     int                          `json:"total"`
	Delivered int                          `json:"delivered"`
	Failed    int                          `json:"failed"`
	Pending   int                          `json:"pending"`
	Skipped   int                          `json:"skipped"`
	Targets   []notificationTargetResponse `json:"targets"`
}

// notificationTargetResponse is a notification target.
type notificationTargetResponse struct {
	Type            string                `json:"type"`
	Name            string                `json:"name"`
	Status          target.DeliveryStatus `json:"status"`
	LastAttemptAt   *time.Time            `json:"lastAttemptAt,omitempty"`
	LastDeliveredAt *time.Time            `json:"lastDeliveredAt,omitempty"`
}

// respondText writes a plain text response.
func (a *API) respondText(w http.ResponseWriter, status int, body string) {
	if err := encodeText(w, status, body); err != nil {
		a.logger.Error("encode text response failed", "error", err)
	}
}

// respondJSON writes a JSON response and logs encoding failures.
func (a *API) respondJSON(w http.ResponseWriter, status int, value any) {
	if err := encodeJSON(w, status, value); err != nil {
		a.logger.Error("encode json response failed", "error", err)
	}
}

// encodeJSON encodes a value to JSON and writes it to the response.
func encodeJSON[T any](w http.ResponseWriter, status int, value T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// encodeText writes a plain text response.
func encodeText(w http.ResponseWriter, status int, body string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("encode text: %w", err)
	}
	return nil
}

// newCheckInResponse builds a compact HTTP response from an accepted check-in.
func newCheckInResponse() checkInResponse {
	return checkInResponse{Status: checkInResponseStatusOK}
}

// newSnapshotResponse builds a compact HTTP response from monitor state.
func newSnapshotResponse(snapshot monitor.Snapshot) snapshotResponse {
	return snapshotResponse{
		LastCheckIn:  utils.PtrIfNonZero(snapshot.LastCheckIn),
		ExpectedBy:   utils.PtrIfNonZero(snapshot.ExpectedBy),
		OverdueSince: utils.PtrIfNonZero(snapshot.OverdueSince),
		AlertingAt:   utils.PtrIfNonZero(snapshot.AlertingAt),
		Phase:        snapshot.Phase,
	}
}

// newAcceptedCheckInDetailsResponse builds a detailed HTTP response from an accepted check-in.
func newAcceptedCheckInDetailsResponse(checkInName string, snapshot monitor.Snapshot, now time.Time) checkInDetailsResponse {
	response := newCheckInDetailsResponse(checkInName, snapshot, now)
	response.Status = checkInResponseStatusOK
	return response
}

// newCheckInDetailsResponse builds a detailed HTTP response from monitor state.
func newCheckInDetailsResponse(checkInName string, snapshot monitor.Snapshot, now time.Time) checkInDetailsResponse {
	response := checkInDetailsResponse{
		CheckInName:  checkInName,
		Phase:        snapshot.Phase,
		LastCheckIn:  utils.PtrIfNonZero(snapshot.LastCheckIn),
		ExpectedBy:   utils.PtrIfNonZero(snapshot.ExpectedBy),
		OverdueSince: utils.PtrIfNonZero(snapshot.OverdueSince),
		AlertingAt:   utils.PtrIfNonZero(snapshot.AlertingAt),
	}

	if !snapshot.LastCheckIn.IsZero() && !snapshot.ExpectedBy.IsZero() {
		response.ExpectedEvery = snapshot.ExpectedBy.Sub(snapshot.LastCheckIn).String()
	}
	if !snapshot.OverdueSince.IsZero() && !now.Before(snapshot.OverdueSince) {
		response.OverdueFor = now.Sub(snapshot.OverdueSince).String()
	}
	if !snapshot.ExpectedBy.IsZero() && !snapshot.AlertingAt.IsZero() {
		response.AlertingDelay = snapshot.AlertingAt.Sub(snapshot.ExpectedBy).String()
	}
	if !snapshot.LastCheckIn.IsZero() && !snapshot.AlertingAt.IsZero() {
		response.AlertingAfter = snapshot.AlertingAt.Sub(snapshot.LastCheckIn).String()
	}
	if !snapshot.AlertingAt.IsZero() && !now.Before(snapshot.AlertingAt) {
		response.AlertingFor = now.Sub(snapshot.AlertingAt).String()
	}

	return response
}

// newNotificationStatusResponse builds a notification delivery status response.
func newNotificationStatusResponse(status target.Status) notificationStatusResponse {
	targets := make([]notificationTargetResponse, 0, len(status.Targets))
	for _, target := range status.Targets {
		targets = append(targets, notificationTargetResponse{
			Type:            target.Type,
			Name:            target.Name,
			Status:          target.Status,
			LastAttemptAt:   target.LastAttemptAt,
			LastDeliveredAt: target.LastDeliveredAt,
		})
	}

	return notificationStatusResponse{
		Status:    status.Status,
		Total:     status.Total,
		Delivered: status.Delivered,
		Failed:    status.Failed,
		Pending:   status.Pending,
		Skipped:   status.Skipped,
		Targets:   targets,
	}
}
