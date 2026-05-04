package http

import (
	"io/fs"
	stdhttp "net/http"

	"sync-exchange-rate/internal/delivery/http/handler"
)

func NewRouter(healthHandler *handler.HealthHandler, syncHandler *handler.SyncHandler, reportHandler *handler.ReportHandler) stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.Handle("/health", healthHandler)
	mux.Handle("/sync", syncHandler)
	mux.Handle("/reports/rates", reportHandler)
	mux.Handle("/", newStaticHandler())
	return mux
}

func newStaticHandler() stdhttp.Handler {
	staticFS, err := fs.Sub(embeddedStaticFiles, "static")
	if err != nil {
		panic(err)
	}

	return stdhttp.FileServer(stdhttp.FS(staticFS))
}
