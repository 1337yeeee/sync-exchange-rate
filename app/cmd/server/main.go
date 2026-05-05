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

	"sync-exchange-rate/internal/app"
	"sync-exchange-rate/internal/config"
	deliveryhttp "sync-exchange-rate/internal/delivery/http"
	"sync-exchange-rate/internal/delivery/http/handler"
	"sync-exchange-rate/internal/scheduler"
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

	components, err := app.NewComponents(context.Background(), cfg, app.DBRetryConfig{
		MaxAttempts: 10,
		Delay:       2 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("init app components: %w", err)
	}
	defer components.Close()

	healthHandler := handler.NewHealthHandler()
	syncHandler := handler.NewSyncHandler(components.SyncService)
	reportHandler := handler.NewReportHandler(components.ReportService)
	router := deliveryhttp.NewRouter(healthHandler, syncHandler, reportHandler)

	var syncScheduler *scheduler.Scheduler
	if cfg.Sync.SchedulerEnabled {
		syncScheduler, err = scheduler.New(components.SyncService, cfg.Sync.Schedule)
		if err != nil {
			return fmt.Errorf("init scheduler: %w", err)
		}
		log.Printf("embedded scheduler enabled: schedule=%q", cfg.Sync.Schedule)
		syncScheduler.Start()
	} else {
		log.Printf("embedded scheduler disabled")
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

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
			stopScheduler(syncScheduler)
			return fmt.Errorf("run http server: %w", serveErr)
		}
	}

	stopScheduler(syncScheduler)

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	log.Printf("server stopped gracefully")
	return nil
}

func stopScheduler(s *scheduler.Scheduler) {
	if s != nil {
		s.Stop()
	}
}
