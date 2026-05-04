package report

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sync-exchange-rate/internal/domain"
)

type fakeRateRepository struct {
	rates []domain.Rate
	err   error
}

func (r *fakeRateRepository) Save(_ context.Context, _ []domain.Rate) error {
	return nil
}

func (r *fakeRateRepository) GetByPeriod(_ context.Context, currencies []string, start, end time.Time) ([]domain.Rate, error) {
	if r.err != nil {
		return nil, r.err
	}

	filter := make(map[string]struct{}, len(currencies))
	for _, currency := range currencies {
		filter[currency] = struct{}{}
	}

	result := make([]domain.Rate, 0, len(r.rates))
	for _, rate := range r.rates {
		if rate.TradingDate.Before(start.UTC()) || rate.TradingDate.After(end.UTC()) {
			continue
		}

		if _, ok := filter[rate.CurrencyCode]; !ok {
			continue
		}

		result = append(result, rate)
	}

	return result, nil
}

func (r *fakeRateRepository) GetExistingDates(_ context.Context, start, end time.Time) (map[time.Time]bool, error) {
	return map[time.Time]bool{}, nil
}

func TestBuildReportCalculatesMinMaxAvg(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{
		rates: []domain.Rate{
			newReportRate(t, "USD", 25.10, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
			newReportRate(t, "USD", 24.90, time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC)),
			newReportRate(t, "USD", 25.40, time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC)),
		},
	})

	report, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC),
		[]string{"USD"},
	)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}

	if len(report.Currencies) != 1 {
		t.Fatalf("len(report.Currencies) = %d, want 1", len(report.Currencies))
	}

	currency := report.Currencies[0]
	assertFloatPtr(t, currency.MinRate, 24.90, "MinRate")
	assertFloatPtr(t, currency.MaxRate, 25.40, "MaxRate")
	assertFloatPtr(t, currency.AvgRate, 25.133333, "AvgRate")

	if currency.Observations != 3 {
		t.Fatalf("Observations = %d, want 3", currency.Observations)
	}
}

func TestBuildReportSupportsMultipleCurrencies(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{
		rates: []domain.Rate{
			newReportRate(t, "USD", 25.10, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
			newReportRate(t, "EUR", 27.20, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
			newReportRate(t, "EUR", 27.50, time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC)),
		},
	})

	report, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC),
		[]string{"USD", "EUR"},
	)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}

	if len(report.Currencies) != 2 {
		t.Fatalf("len(report.Currencies) = %d, want 2", len(report.Currencies))
	}

	if report.Currencies[0].CurrencyCode != "USD" || report.Currencies[1].CurrencyCode != "EUR" {
		t.Fatalf("unexpected order or currency codes: %+v", report.Currencies)
	}
}

func TestBuildReportReturnsCurrencyWithoutData(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{
		rates: []domain.Rate{
			newReportRate(t, "USD", 25.10, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
		},
	})

	report, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC),
		[]string{"USD", "EUR"},
	)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}

	eur := report.Currencies[1]
	if eur.CurrencyCode != "EUR" {
		t.Fatalf("CurrencyCode = %q, want EUR", eur.CurrencyCode)
	}

	if eur.MinRate != nil || eur.MaxRate != nil || eur.AvgRate != nil {
		t.Fatalf("expected nil metrics for EUR, got %+v", eur)
	}

	if eur.Observations != 0 {
		t.Fatalf("Observations = %d, want 0", eur.Observations)
	}
}

func TestBuildReportHandlesMissingDates(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{
		rates: []domain.Rate{
			newReportRate(t, "USD", 25.10, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)),
			newReportRate(t, "USD", 24.90, time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC)),
		},
	})

	report, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC),
		[]string{"USD"},
	)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}

	currency := report.Currencies[0]
	if currency.Observations != 2 {
		t.Fatalf("Observations = %d, want 2", currency.Observations)
	}

	assertFloatPtr(t, currency.AvgRate, 25.00, "AvgRate")
}

func TestBuildReportRejectsEmptyPeriod(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{})

	_, err := service.BuildReport(context.Background(), time.Time{}, time.Time{}, []string{"USD"})
	if err == nil {
		t.Fatal("BuildReport() error = nil, want validation error")
	}
}

func TestBuildReportRejectsInvalidDateRange(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{})

	_, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		[]string{"USD"},
	)
	if err == nil {
		t.Fatal("BuildReport() error = nil, want validation error")
	}
}

func TestBuildReportUsesNormalizedRate(t *testing.T) {
	t.Parallel()

	huf := domain.Rate{
		TradingDate:  time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "HUF",
		Country:      "Hungary",
		CurrencyName: "forint",
		Amount:       100,
		Rate:         8.012,
	}
	if err := huf.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	service := mustNewReportService(t, &fakeRateRepository{
		rates: []domain.Rate{huf},
	})

	report, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		[]string{"HUF"},
	)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}

	assertFloatPtr(t, report.Currencies[0].AvgRate, 0.08012, "AvgRate")
}

func TestBuildReportReturnsRepositoryError(t *testing.T) {
	t.Parallel()

	service := mustNewReportService(t, &fakeRateRepository{
		err: fmt.Errorf("db unavailable"),
	})

	_, err := service.BuildReport(
		context.Background(),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		[]string{"USD"},
	)
	if err == nil {
		t.Fatal("BuildReport() error = nil, want repository error")
	}
}

func mustNewReportService(t *testing.T, repo *fakeRateRepository) *Service {
	t.Helper()

	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func newReportRate(t *testing.T, code string, normalizedRate float64, tradingDate time.Time) domain.Rate {
	t.Helper()

	rate := domain.Rate{
		TradingDate:    tradingDate,
		CurrencyCode:   code,
		Country:        code + " country",
		CurrencyName:   code + " currency",
		Amount:         1,
		Rate:           normalizedRate,
		NormalizedRate: normalizedRate,
	}

	if err := rate.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	return rate
}

func assertFloatPtr(t *testing.T, got *float64, want float64, name string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s = nil, want %v", name, want)
	}

	if *got != want {
		t.Fatalf("%s = %v, want %v", name, *got, want)
	}
}
