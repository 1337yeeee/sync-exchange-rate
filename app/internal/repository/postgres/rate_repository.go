package postgres

import (
	"context"
	"gorm.io/gorm"
	"sync-exchange-rate/internal/domain"
	"sync-exchange-rate/internal/repository"
	"time"
)

type rateRepository struct {
	db *gorm.DB
}

func NewRateRepository(db *gorm.DB) repository.RateRepository {
	return &rateRepository{db: db}
}

func (r *rateRepository) Save(ctx context.Context, rates []domain.Rate) error {
	return nil
}

func (r *rateRepository) GetByPeriod(ctx context.Context, currencies []string, start, end time.Time) ([]domain.Rate, error) {
	return nil, nil
}

func (r *rateRepository) GetExistingDates(ctx context.Context, start, end time.Time) (map[time.Time]bool, error) {
	return nil, nil
}
