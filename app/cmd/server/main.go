package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cnbclient "sync-exchange-rate/internal/client/cnb"
	"sync-exchange-rate/internal/config"
	deliveryhttp "sync-exchange-rate/internal/delivery/http"
	"sync-exchange-rate/internal/delivery/http/handler"
	raterepository "sync-exchange-rate/internal/repository/postgres"
	"sync-exchange-rate/internal/scheduler"
	reportservice "sync-exchange-rate/internal/service/report"
	syncservice "sync-exchange-rate/internal/service/sync"
	"sync-exchange-rate/internal/storage/database"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := database.NewPostgres(cfg)
	if err != nil {
		return fmt.Errorf("init postgres: %w", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	cnb, err := cnbclient.NewClient(cfg.CNB.BaseURL, nil)
	if err != nil {
		return fmt.Errorf("init cnb client: %w", err)
	}

	rateRepository := raterepository.NewRateRepository(db)

	syncSvc, err := syncservice.NewService(cnb, rateRepository, cfg.Sync.Currencies)
	if err != nil {
		return fmt.Errorf("init sync service: %w", err)
	}

	reportSvc, err := reportservice.NewService(rateRepository)
	if err != nil {
		return fmt.Errorf("init report service: %w", err)
	}

	healthHandler := handler.NewHealthHandler()
	syncHandler := handler.NewSyncHandler(syncSvc)
	reportHandler := handler.NewReportHandler(reportSvc)
	router := deliveryhttp.NewRouter(healthHandler, syncHandler, reportHandler)

	syncScheduler, err := scheduler.New(syncSvc, cfg.Sync.Schedule)
	if err != nil {
		return fmt.Errorf("init scheduler: %w", err)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	syncScheduler.Start()

	serverErrCh := make(chan error, 1)
	go func() {
		log.Printf("http server listening on %s", server.Addr)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErrCh <- serveErr
		}
		close(serverErrCh)
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-shutdownCtx.Done():
		log.Printf("shutdown signal received")
	case serveErr, ok := <-serverErrCh:
		if ok && serveErr != nil {
			syncScheduler.Stop()
			return fmt.Errorf("run http server: %w", serveErr)
		}
	}

	syncScheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	log.Printf("server stopped gracefully")
	return nil
}
