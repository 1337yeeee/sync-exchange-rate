package http

import (
	stdhttp "net/http"

	"sync-exchange-rate/internal/delivery/http/handler"
)

func NewRouter(healthHandler *handler.HealthHandler, syncHandler *handler.SyncHandler, reportHandler *handler.ReportHandler) stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.Handle("/health", healthHandler)
	mux.Handle("/sync", syncHandler)
	mux.Handle("/reports/rates", reportHandler)
	return mux
}
