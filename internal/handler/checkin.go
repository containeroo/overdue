package handler

import "net/http"

// CheckIn returns the check-in endpoint handler.
func (a *API) CheckIn() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := requestMetadata(r)

		if !a.authorized(r) {
			a.logger.Error("unauthorized check-in request", request.LogFields()...)
			a.respondJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}

		now := a.nowFn()
		result := a.service.RecordCheckIn(r.Context(), now)
		a.logCheckInReceived(result, now, request)

		if a.wantsDetails(r) {
			a.respondJSON(w, http.StatusOK, newAcceptedCheckInDetailsResponse(result.CheckInName, result.Snapshot, now))
			return
		}

		a.respondJSON(w, http.StatusOK, newCheckInResponse())
	}
}
