package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sync-exchange-rate/internal/domain"
	"sync-exchange-rate/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type rateRepository struct {
	db *gorm.DB
}

func NewRateRepository(db *gorm.DB) repository.RateRepository {
	return &rateRepository{db: db}
}

func (r *rateRepository) Save(ctx context.Context, rates []domain.Rate) error {
	if r.db == nil {
		return fmt.Errorf("postgres db is not configured")
	}

	if len(rates) == 0 {
		return nil
	}

	db := r.db.WithContext(ctx).Model(&domain.Rate{})

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "trading_date"},
			{Name: "currency_code"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"country",
			"currency_name",
			"amount",
			"rate",
			"normalized_rate",
			"updated_at",
		}),
	}).Create(&rates).Error
}

func (r *rateRepository) GetByPeriod(ctx context.Context, currencies []string, start, end time.Time) ([]domain.Rate, error) {
	if r.db == nil {
		return nil, fmt.Errorf("postgres db is not configured")
	}

	var rates []domain.Rate
	query := r.db.WithContext(ctx).Model(&domain.Rate{})

	if !start.IsZero() {
		query = query.Where("trading_date >= ?", start.UTC())
	}

	if !end.IsZero() {
		query = query.Where("trading_date <= ?", end.UTC())
	}

	filter := normalizeCurrencies(currencies)
	if len(filter) > 0 {
		query = query.Where("currency_code IN ?", filter)
	}

	if err := query.Order("trading_date ASC").Order("currency_code ASC").Find(&rates).Error; err != nil {
		return nil, err
	}

	return rates, nil
}

func (r *rateRepository) GetExistingDates(ctx context.Context, start, end time.Time) (map[time.Time]bool, error) {
	if r.db == nil {
		return nil, fmt.Errorf("postgres db is not configured")
	}

	var dates []time.Time
	query := r.db.WithContext(ctx).Model(&domain.Rate{}).Distinct("trading_date")

	if !start.IsZero() {
		query = query.Where("trading_date >= ?", start.UTC())
	}

	if !end.IsZero() {
		query = query.Where("trading_date <= ?", end.UTC())
	}

	if err := query.Order("trading_date ASC").Pluck("trading_date", &dates).Error; err != nil {
		return nil, err
	}

	result := make(map[time.Time]bool, len(dates))
	for _, date := range dates {
		result[date.UTC()] = true
	}

	return result, nil
}

func normalizeCurrencies(currencies []string) []string {
	if len(currencies) == 0 {
		return nil
	}

	result := make([]string, 0, len(currencies))
	seen := make(map[string]struct{}, len(currencies))

	for _, currency := range currencies {
		code := strings.ToUpper(strings.TrimSpace(currency))
		if code == "" {
			continue
		}

		if _, exists := seen[code]; exists {
			continue
		}

		seen[code] = struct{}{}
		result = append(result, code)
	}

	return result
}
