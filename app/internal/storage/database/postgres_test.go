package database

import (
	"testing"
	"time"

	"sync-exchange-rate/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateCreatesRateSchema(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	if !db.Migrator().HasTable(&domain.Rate{}) {
		t.Fatal("rates table was not created")
	}

	if !db.Migrator().HasColumn(&domain.Rate{}, "normalized_rate") {
		t.Fatal("normalized_rate column was not created")
	}
}

func TestAutoMigrateRejectsNilDB(t *testing.T) {
	if err := AutoMigrate(nil); err == nil {
		t.Fatal("AutoMigrate(nil) error = nil, want validation error")
	}
}

func TestRateUniqueConstraintByTradingDateAndCurrencyCode(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	first := domain.Rate{
		TradingDate:  time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		Country:      "United States",
		CurrencyName: "dollar",
		CurrencyCode: "USD",
		Amount:       1,
		Rate:         23.45,
	}

	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("db.Create(first) error = %v", err)
	}

	duplicate := domain.Rate{
		TradingDate:  first.TradingDate,
		Country:      "United States",
		CurrencyName: "dollar",
		CurrencyCode: "usd",
		Amount:       1,
		Rate:         23.99,
	}

	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("db.Create(duplicate) error = nil, want unique constraint violation")
	}

	var count int64
	if err := db.Model(&domain.Rate{}).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	return db
}
