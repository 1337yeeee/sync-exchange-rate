package domain

import (
	"testing"
	"time"
)

func TestRateNormalizeCalculatesNormalizedRate(t *testing.T) {
	tradingDate := time.Date(2024, time.March, 10, 12, 30, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	rate := &Rate{
		TradingDate:  tradingDate,
		CurrencyCode: " usd ",
		Amount:       100,
		Rate:         256.4,
	}

	if err := rate.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if rate.NormalizedRate != 2.564 {
		t.Fatalf("NormalizedRate = %v, want 2.564", rate.NormalizedRate)
	}

	if rate.CurrencyCode != "USD" {
		t.Fatalf("CurrencyCode = %q, want USD", rate.CurrencyCode)
	}

	if rate.TradingDate.Location() != time.UTC {
		t.Fatalf("TradingDate location = %v, want UTC", rate.TradingDate.Location())
	}
}

func TestRateNormalizeRejectsNonPositiveAmount(t *testing.T) {
	rate := &Rate{
		TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "USD",
		Amount:       0,
		Rate:         25.64,
	}

	if err := rate.Normalize(); err == nil {
		t.Fatal("Normalize() error = nil, want validation error")
	}
}

func TestRateNormalizeRejectsMissingTradingDate(t *testing.T) {
	rate := &Rate{
		CurrencyCode: "USD",
		Amount:       1,
		Rate:         25.64,
	}

	if err := rate.Normalize(); err == nil {
		t.Fatal("Normalize() error = nil, want validation error")
	}
}

func TestRateNormalizeRejectsMissingCurrencyCode(t *testing.T) {
	rate := &Rate{
		TradingDate: time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
		Amount:      1,
		Rate:        25.64,
	}

	if err := rate.Normalize(); err == nil {
		t.Fatal("Normalize() error = nil, want validation error")
	}
}
