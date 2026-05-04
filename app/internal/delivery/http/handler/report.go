package handler

import (
	"context"
	"net/http"
	"time"

	"sync-exchange-rate/internal/delivery/http/dto"
	reportservice "sync-exchange-rate/internal/service/report"
)

type reportService interface {
	BuildReport(ctx context.Context, startDate, endDate time.Time, currencies []string) (reportservice.Report, error)
}

type ReportHandler struct {
	service reportService
}

func NewReportHandler(service reportService) *ReportHandler {
	return &ReportHandler{service: service}
}

func (h *ReportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	request, err := dto.ParseReportRequest(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.service.BuildReport(r.Context(), request.StartDate, request.EndDate, request.Currencies)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dto.NewReportResponse(report))
}
