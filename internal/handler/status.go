package handler

import "net/http"

// Status returns the current check-in monitor status endpoint handler.
func (a *API) Status() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := requestMetadata(r)

		if !a.authorized(r) {
			a.logger.Error("unauthorized status request", request.LogFields()...)
			a.respondJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}

		now := a.nowFn()
		snapshotResult := a.service.Snapshot()
		a.logSnapshotRequested(snapshotResult, request)

		if a.wantsDetails(r) {
			response := newCheckInDetailsResponse(snapshotResult.CheckInName, snapshotResult.Snapshot, now)
			if snapshotResult.HasNotificationStatus {
				notifications := newNotificationStatusResponse(snapshotResult.NotificationStatus)
				response.Notifications = &notifications
			}
			a.respondJSON(w, http.StatusOK, response)
			return
		}

		a.respondJSON(w, http.StatusOK, newSnapshotResponse(snapshotResult.Snapshot))
	}
}
