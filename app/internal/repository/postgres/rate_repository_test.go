package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sync-exchange-rate/internal/domain"
	"sync-exchange-rate/internal/storage/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSavePersistsNewRates(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	rates := []domain.Rate{
		newRate(t, "USD", 1, 22.50, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
		newRate(t, "EUR", 1, 24.10, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
	}

	if err := repo.Save(context.Background(), rates); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	saved := fetchAllRates(t, repo.db)
	if len(saved) != 2 {
		t.Fatalf("len(saved) = %d, want 2", len(saved))
	}
}

func TestSaveSameDateAndCurrencyDoesNotCreateDuplicates(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	tradingDate := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	first := newRate(t, "USD", 1, 22.50, tradingDate)
	second := newRate(t, "USD", 1, 22.50, tradingDate)

	if err := repo.Save(context.Background(), []domain.Rate{first}); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}

	if err := repo.Save(context.Background(), []domain.Rate{second}); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}

	var count int64
	if err := repo.db.Model(&domain.Rate{}).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestSaveUpdatesExistingRate(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	tradingDate := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	original := newRate(t, "USD", 1, 22.50, tradingDate)
	updated := newRate(t, "USD", 1, 23.75, tradingDate)
	updated.Country = "United States of America"

	if err := repo.Save(context.Background(), []domain.Rate{original}); err != nil {
		t.Fatalf("Save(original) error = %v", err)
	}

	if err := repo.Save(context.Background(), []domain.Rate{updated}); err != nil {
		t.Fatalf("Save(updated) error = %v", err)
	}

	saved := fetchAllRates(t, repo.db)
	if len(saved) != 1 {
		t.Fatalf("len(saved) = %d, want 1", len(saved))
	}

	if saved[0].Rate != 23.75 {
		t.Fatalf("saved rate = %v, want 23.75", saved[0].Rate)
	}

	if saved[0].NormalizedRate != 23.75 {
		t.Fatalf("saved normalized rate = %v, want 23.75", saved[0].NormalizedRate)
	}

	if saved[0].Country != "United States of America" {
		t.Fatalf("saved country = %q", saved[0].Country)
	}
}

func TestGetByPeriodFiltersDates(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	seedRates(t, repo, []domain.Rate{
		newRate(t, "USD", 1, 22.50, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
		newRate(t, "USD", 1, 22.60, time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC)),
		newRate(t, "USD", 1, 22.70, time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC)),
	})

	rates, err := repo.GetByPeriod(
		context.Background(),
		nil,
		time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("GetByPeriod() error = %v", err)
	}

	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}
}

func TestGetByPeriodFiltersCurrencies(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	seedRates(t, repo, []domain.Rate{
		newRate(t, "USD", 1, 22.50, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
		newRate(t, "EUR", 1, 24.10, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
		newRate(t, "GBP", 1, 28.00, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
	})

	rates, err := repo.GetByPeriod(
		context.Background(),
		[]string{"usd", "eur", "usd"},
		time.Time{},
		time.Time{},
	)
	if err != nil {
		t.Fatalf("GetByPeriod() error = %v", err)
	}

	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}

	if rates[0].CurrencyCode != "EUR" || rates[1].CurrencyCode != "USD" {
		t.Fatalf("unexpected codes: %s, %s", rates[0].CurrencyCode, rates[1].CurrencyCode)
	}
}

func TestGetByPeriodReturnsEmptyResult(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)

	rates, err := repo.GetByPeriod(
		context.Background(),
		[]string{"USD"},
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("GetByPeriod() error = %v", err)
	}

	if len(rates) != 0 {
		t.Fatalf("len(rates) = %d, want 0", len(rates))
	}
}

func TestGetExistingDatesReturnsDistinctDates(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	firstDate := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	secondDate := time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC)

	seedRates(t, repo, []domain.Rate{
		newRate(t, "USD", 1, 22.50, firstDate),
		newRate(t, "EUR", 1, 24.10, firstDate),
		newRate(t, "USD", 1, 22.60, secondDate),
	})

	dates, err := repo.GetExistingDates(context.Background(), firstDate, secondDate)
	if err != nil {
		t.Fatalf("GetExistingDates() error = %v", err)
	}

	if len(dates) != 2 {
		t.Fatalf("len(dates) = %d, want 2", len(dates))
	}

	if !dates[firstDate] || !dates[secondDate] {
		t.Fatalf("expected both dates in result, got %v", dates)
	}
}

func TestRepositoryReturnsDBError(t *testing.T) {
	t.Parallel()

	repo := newTestRateRepository(t)
	sqlDB, err := repo.db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		t.Fatalf("sqlDB.Close() error = %v", err)
	}

	err = repo.Save(context.Background(), []domain.Rate{
		newRate(t, "USD", 1, 22.50, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
	})
	if err == nil {
		t.Fatal("Save() error = nil, want db error")
	}
}

func newTestRateRepository(t *testing.T) *rateRepository {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	return &rateRepository{db: db}
}

func newRate(t *testing.T, code string, amount int, rateValue float64, tradingDate time.Time) domain.Rate {
	t.Helper()

	rate := domain.Rate{
		TradingDate:  tradingDate,
		Country:      code + " country",
		CurrencyName: code + " currency",
		CurrencyCode: code,
		Amount:       amount,
		Rate:         rateValue,
	}

	if err := rate.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	return rate
}

func seedRates(t *testing.T, repo *rateRepository, rates []domain.Rate) {
	t.Helper()

	if err := repo.Save(context.Background(), rates); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func fetchAllRates(t *testing.T, db *gorm.DB) []domain.Rate {
	t.Helper()

	var rates []domain.Rate
	if err := db.Order("trading_date ASC").Order("currency_code ASC").Find(&rates).Error; err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	return rates
}
