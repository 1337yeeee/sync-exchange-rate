package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	cnbclient "sync-exchange-rate/internal/client/cnb"
	"sync-exchange-rate/internal/config"
	"sync-exchange-rate/internal/repository"
	raterepository "sync-exchange-rate/internal/repository/postgres"
	reportservice "sync-exchange-rate/internal/service/report"
	syncservice "sync-exchange-rate/internal/service/sync"
	"sync-exchange-rate/internal/storage/database"

	"gorm.io/gorm"
)

type Components struct {
	DB             *gorm.DB
	RateRepository repository.RateRepository
	SyncService    *syncservice.Service
	ReportService  *reportservice.Service
}

type DBRetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

func NewComponents(ctx context.Context, cfg config.Config, retry DBRetryConfig) (*Components, error) {
	db, err := connectPostgresWithRetry(ctx, cfg, retry)
	if err != nil {
		return nil, err
	}

	if err := database.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	cnb, err := cnbclient.NewClient(cfg.CNB.BaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("init cnb client: %w", err)
	}

	rateRepository := raterepository.NewRateRepository(db)

	syncSvc, err := syncservice.NewService(cnb, rateRepository, cfg.Sync.Currencies)
	if err != nil {
		return nil, fmt.Errorf("init sync service: %w", err)
	}

	reportSvc, err := reportservice.NewService(rateRepository)
	if err != nil {
		return nil, fmt.Errorf("init report service: %w", err)
	}

	return &Components{
		DB:             db,
		RateRepository: rateRepository,
		SyncService:    syncSvc,
		ReportService:  reportSvc,
	}, nil
}

func (c *Components) Close() {
	if c == nil || c.DB == nil {
		return
	}

	closeDB(c.DB)
}

func closeDB(db interface {
	DB() (*sql.DB, error)
}) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("close postgres: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("close postgres: %v", err)
	}
}

func connectPostgresWithRetry(ctx context.Context, cfg config.Config, retry DBRetryConfig) (*gorm.DB, error) {
	maxAttempts := retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	delay := retry.Delay
	if delay <= 0 {
		delay = time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err := database.NewPostgres(cfg)
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				err = sqlDB.PingContext(ctx)
			} else {
				err = dbErr
			}
		}

		if err == nil {
			log.Printf("postgres is ready: attempt=%d", attempt)
			return db, nil
		}

		lastErr = err
		log.Printf("postgres is not ready: attempt=%d/%d error=%v", attempt, maxAttempts, err)

		if attempt == maxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait postgres readiness: %w", ctx.Err())
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("postgres is not ready after %d attempts: %w", maxAttempts, lastErr)
}
