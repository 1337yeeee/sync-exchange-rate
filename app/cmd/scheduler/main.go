package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sync-exchange-rate/internal/app"
	"sync-exchange-rate/internal/config"
	"sync-exchange-rate/internal/scheduler"
)

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

	log.Printf("scheduler runtime starting: schedule=%q currencies=%v", cfg.Sync.Schedule, cfg.Sync.Currencies)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	components, err := app.NewComponents(ctx, cfg, app.DBRetryConfig{
		MaxAttempts: 30,
		Delay:       2 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("init scheduler components: %w", err)
	}
	defer components.Close()

	syncScheduler, err := scheduler.New(components.SyncService, cfg.Sync.Schedule)
	if err != nil {
		return fmt.Errorf("init scheduler: %w", err)
	}

	syncScheduler.Start()
	log.Printf("scheduler runtime started")

	<-ctx.Done()
	log.Printf("scheduler shutdown signal received")

	syncScheduler.Stop()
	log.Printf("scheduler stopped gracefully")

	return nil
}
