package cnb

import (
	"testing"
	"time"

	"sync-exchange-rate/internal/domain"
)

const validYearFixture = `Date|1 EUR|1 USD|100 HUF|100 JPY
02.01.2019|25.725|22.709|8.012|20.836
03.01.2019|25.665|22.671|8.005|21.012
Date|1 EUR|1 USD|100 HUF|100 JPY|1 GBP
04.01.2019|25.575|22.550|8.000|21.100|28.350
`

func TestParseYearValidFile(t *testing.T) {
	t.Parallel()

	rates, err := ParseYear(validYearFixture, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ParseYear() error = %v", err)
	}

	if len(rates) != 13 {
		t.Fatalf("len(rates) = %d, want 13", len(rates))
	}

	assertRate(t, rates[0], "EUR", "", "EUR", 1, 25.725, 25.725, time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC))
	assertRate(t, rates[12], "GBP", "", "GBP", 1, 28.350, 28.350, time.Date(2019, time.January, 4, 0, 0, 0, 0, time.UTC))
}

func TestParseYearFiltersCurrencies(t *testing.T) {
	t.Parallel()

	rates, err := ParseYear(validYearFixture, []string{"usd", "eur"}, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ParseYear() error = %v", err)
	}

	if len(rates) != 6 {
		t.Fatalf("len(rates) = %d, want 6", len(rates))
	}

	for _, rate := range rates {
		if rate.CurrencyCode != "USD" && rate.CurrencyCode != "EUR" {
			t.Fatalf("unexpected currency code %q", rate.CurrencyCode)
		}
	}
}

func TestParseYearSkipsMissingValues(t *testing.T) {
	t.Parallel()

	raw := `Date|1 EUR|1 USD|100 HUF
02.01.2019|25.725||8.012
03.01.2019|25.665|22.671
`

	rates, err := ParseYear(raw, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ParseYear() error = %v", err)
	}

	if len(rates) != 4 {
		t.Fatalf("len(rates) = %d, want 4", len(rates))
	}

	codes := []string{rates[0].CurrencyCode, rates[1].CurrencyCode, rates[2].CurrencyCode, rates[3].CurrencyCode}
	want := []string{"EUR", "HUF", "EUR", "USD"}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("codes[%d] = %q, want %q; all codes = %v", i, codes[i], want[i], codes)
		}
	}
}

func TestParseYearFiltersDateRange(t *testing.T) {
	t.Parallel()

	start := time.Date(2019, time.January, 3, 0, 0, 0, 0, time.UTC)
	end := time.Date(2019, time.January, 3, 0, 0, 0, 0, time.UTC)

	rates, err := ParseYear(validYearFixture, []string{"EUR", "USD"}, start, end)
	if err != nil {
		t.Fatalf("ParseYear() error = %v", err)
	}

	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}

	for _, rate := range rates {
		if !rate.TradingDate.Equal(start) {
			t.Fatalf("TradingDate = %v, want %v", rate.TradingDate, start)
		}
	}
}

func TestParseYearSkipsInvalidNumbers(t *testing.T) {
	t.Parallel()

	raw := `Date|1 EUR|1 USD|100 HUF
02.01.2019|bad|22.709|8.012
03.01.2019|25.665|oops|8.005
`

	rates, err := ParseYear(raw, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ParseYear() error = %v", err)
	}

	if len(rates) != 4 {
		t.Fatalf("len(rates) = %d, want 4", len(rates))
	}

	for _, rate := range rates {
		if rate.CurrencyCode == "EUR" && rate.Rate == 0 {
			t.Fatal("invalid EUR rate was not skipped")
		}
	}
}

func TestParseYearHandlesAmountGreaterThanOne(t *testing.T) {
	t.Parallel()

	rates, err := ParseYear(validYearFixture, []string{"HUF", "JPY"}, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ParseYear() error = %v", err)
	}

	if len(rates) != 6 {
		t.Fatalf("len(rates) = %d, want 6", len(rates))
	}

	assertRate(t, rates[0], "HUF", "", "HUF", 100, 8.012, 0.08012, time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC))
	assertRate(t, rates[1], "JPY", "", "JPY", 100, 20.836, 0.20836, time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC))
}

func TestParseYearRejectsEmptyFile(t *testing.T) {
	t.Parallel()

	_, err := ParseYear(" \n \r\n ", nil, time.Time{}, time.Time{})
	if err == nil {
		t.Fatal("ParseYear() error = nil, want empty response error")
	}
}

func assertRate(t *testing.T, got domain.Rate, code, country, currencyName string, amount int, rateValue, normalizedRate float64, tradingDate time.Time) {
	t.Helper()

	if got.CurrencyCode != code {
		t.Fatalf("CurrencyCode = %q, want %q", got.CurrencyCode, code)
	}

	if got.Country != country {
		t.Fatalf("Country = %q, want %q", got.Country, country)
	}

	if got.CurrencyName != currencyName {
		t.Fatalf("CurrencyName = %q, want %q", got.CurrencyName, currencyName)
	}

	if got.Amount != amount {
		t.Fatalf("Amount = %d, want %d", got.Amount, amount)
	}

	if got.Rate != rateValue {
		t.Fatalf("Rate = %v, want %v", got.Rate, rateValue)
	}

	if got.NormalizedRate != normalizedRate {
		t.Fatalf("NormalizedRate = %v, want %v", got.NormalizedRate, normalizedRate)
	}

	if !got.TradingDate.Equal(tradingDate) {
		t.Fatalf("TradingDate = %v, want %v", got.TradingDate, tradingDate)
	}
}
