package sync

import (
	"testing"
	"time"

	"sync-exchange-rate/internal/domain"
)

func TestNormalizeRateWithAmountOne(t *testing.T) {
	t.Parallel()

	rate := domain.Rate{
		TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "usd",
		Amount:       1,
		Rate:         25.64,
	}

	if err := NormalizeRate(&rate); err != nil {
		t.Fatalf("NormalizeRate() error = %v", err)
	}

	if rate.NormalizedRate != 25.64 {
		t.Fatalf("NormalizedRate = %v, want 25.64", rate.NormalizedRate)
	}

	if rate.CurrencyCode != "USD" {
		t.Fatalf("CurrencyCode = %q, want USD", rate.CurrencyCode)
	}
}

func TestNormalizeRateWithAmountHundred(t *testing.T) {
	t.Parallel()

	rate := domain.Rate{
		TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "JPY",
		Amount:       100,
		Rate:         17.345,
	}

	if err := NormalizeRate(&rate); err != nil {
		t.Fatalf("NormalizeRate() error = %v", err)
	}

	if rate.NormalizedRate != 0.17345 {
		t.Fatalf("NormalizedRate = %v, want 0.17345", rate.NormalizedRate)
	}
}

func TestNormalizeRateRejectsZeroAmount(t *testing.T) {
	t.Parallel()

	rate := domain.Rate{
		TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "EUR",
		Amount:       0,
		Rate:         25.64,
	}

	if err := NormalizeRate(&rate); err == nil {
		t.Fatal("NormalizeRate() error = nil, want validation error")
	}
}

func TestNormalizeRateRoundsToSixDecimals(t *testing.T) {
	t.Parallel()

	rate := domain.Rate{
		TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "HUF",
		Amount:       3,
		Rate:         1,
	}

	if err := NormalizeRate(&rate); err != nil {
		t.Fatalf("NormalizeRate() error = %v", err)
	}

	if rate.NormalizedRate != 0.333333 {
		t.Fatalf("NormalizedRate = %v, want 0.333333", rate.NormalizedRate)
	}
}

func TestNormalizeRatesNormalizesEachItem(t *testing.T) {
	t.Parallel()

	rates := []domain.Rate{
		{
			TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
			CurrencyCode: "usd",
			Amount:       1,
			Rate:         25.64,
		},
		{
			TradingDate:  time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC),
			CurrencyCode: "jpy",
			Amount:       100,
			Rate:         17.345,
		},
	}

	if err := NormalizeRates(rates); err != nil {
		t.Fatalf("NormalizeRates() error = %v", err)
	}

	if rates[0].NormalizedRate != 25.64 {
		t.Fatalf("rates[0].NormalizedRate = %v, want 25.64", rates[0].NormalizedRate)
	}

	if rates[1].NormalizedRate != 0.17345 {
		t.Fatalf("rates[1].NormalizedRate = %v, want 0.17345", rates[1].NormalizedRate)
	}
}
