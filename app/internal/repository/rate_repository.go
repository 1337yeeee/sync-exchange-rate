package repository

import (
	"context"
	"sync-exchange-rate/internal/domain"
	"time"
)

type RateRepository interface {
	Save(ctx context.Context, rates []domain.Rate) error
	GetByPeriod(ctx context.Context, currencies []string, start, end time.Time) ([]domain.Rate, error)
	GetExistingDates(ctx context.Context, start, end time.Time) (map[time.Time]bool, error)
}
