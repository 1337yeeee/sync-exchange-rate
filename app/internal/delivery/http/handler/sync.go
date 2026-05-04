package handler

import (
	"context"
	"net/http"
	"time"

	"sync-exchange-rate/internal/delivery/http/dto"
	syncservice "sync-exchange-rate/internal/service/sync"
)

type syncService interface {
	SyncDate(ctx context.Context, date time.Time) (syncservice.Result, error)
	SyncPeriod(ctx context.Context, startDate, endDate time.Time) (syncservice.Result, error)
}

type SyncHandler struct {
	service syncService
}

type SyncResponse struct {
	SavedCount   int      `json:"savedCount"`
	SkippedCount int      `json:"skippedCount"`
	Errors       []string `json:"errors"`
}

func NewSyncHandler(service syncService) *SyncHandler {
	return &SyncHandler{service: service}
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	request, err := dto.ParseSyncRequest(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if request.EndDate == nil {
		result, syncErr := h.service.SyncDate(r.Context(), request.StartDate)
		if syncErr != nil {
			writeError(w, http.StatusInternalServerError, syncErr.Error())
			return
		}

		writeJSON(w, http.StatusOK, SyncResponse(result))
		return
	}

	result, syncErr := h.service.SyncPeriod(r.Context(), request.StartDate, *request.EndDate)
	if syncErr != nil {
		writeError(w, http.StatusInternalServerError, syncErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, SyncResponse(result))
}
