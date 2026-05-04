package cnb

import (
	"testing"
	"time"
)

const validDailyFixture = `26 Jul 2019 #143
Country|Currency|Amount|Code|Rate
EMU|euro|1|EUR|25.540
Hungary|forint|100|HUF|7.824
Japan|yen|100|JPY|21.097
USA|dollar|1|USD|22.932
`

func TestParseDailyValidFile(t *testing.T) {
	t.Parallel()

	rates, err := ParseDaily(validDailyFixture, nil)
	if err != nil {
		t.Fatalf("ParseDaily() error = %v", err)
	}

	if len(rates) != 4 {
		t.Fatalf("len(rates) = %d, want 4", len(rates))
	}

	assertRate(t, rates[0], "EUR", "EMU", "euro", 1, 25.540, 25.540, time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	assertRate(t, rates[3], "USD", "USA", "dollar", 1, 22.932, 22.932, time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
}

func TestParseDailyFiltersCurrencies(t *testing.T) {
	t.Parallel()

	rates, err := ParseDaily(validDailyFixture, []string{"usd", "eur"})
	if err != nil {
		t.Fatalf("ParseDaily() error = %v", err)
	}

	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}

	if rates[0].CurrencyCode != "EUR" || rates[1].CurrencyCode != "USD" {
		t.Fatalf("codes = %s, %s; want EUR, USD", rates[0].CurrencyCode, rates[1].CurrencyCode)
	}
}

func TestParseDailyHandlesAmountGreaterThanOne(t *testing.T) {
	t.Parallel()

	rates, err := ParseDaily(validDailyFixture, []string{"HUF", "JPY"})
	if err != nil {
		t.Fatalf("ParseDaily() error = %v", err)
	}

	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}

	assertRate(t, rates[0], "HUF", "Hungary", "forint", 100, 7.824, 0.07824, time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	assertRate(t, rates[1], "JPY", "Japan", "yen", 100, 21.097, 0.21097, time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
}

func TestParseDailyRejectsEmptyFile(t *testing.T) {
	t.Parallel()

	_, err := ParseDaily(" \n \r\n ", nil)
	if err == nil {
		t.Fatal("ParseDaily() error = nil, want empty response error")
	}
}

func TestParseDailySkipsBrokenLines(t *testing.T) {
	t.Parallel()

	raw := `26 Jul 2019 #143
Country|Currency|Amount|Code|Rate
broken
USA|dollar|1|USD|22.932
Japan|yen|abc|JPY|21.097
Hungary|forint|100|HUF|bad
EMU|euro|1|EUR|25.540
`

	rates, err := ParseDaily(raw, nil)
	if err != nil {
		t.Fatalf("ParseDaily() error = %v", err)
	}

	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}

	if rates[0].CurrencyCode != "USD" || rates[1].CurrencyCode != "EUR" {
		t.Fatalf("codes = %s, %s; want USD, EUR", rates[0].CurrencyCode, rates[1].CurrencyCode)
	}
}

func TestParseDailyUnknownCurrencyIsAllowedWithoutFilter(t *testing.T) {
	t.Parallel()

	raw := `26 Jul 2019 #143
Country|Currency|Amount|Code|Rate
Nowhere|test currency|1|ZZZ|1.234
`

	rates, err := ParseDaily(raw, nil)
	if err != nil {
		t.Fatalf("ParseDaily() error = %v", err)
	}

	if len(rates) != 1 {
		t.Fatalf("len(rates) = %d, want 1", len(rates))
	}

	if rates[0].CurrencyCode != "ZZZ" {
		t.Fatalf("CurrencyCode = %q, want ZZZ", rates[0].CurrencyCode)
	}
}

func TestParseDailyMissingRequestedCurrencyReturnsAvailableRates(t *testing.T) {
	t.Parallel()

	rates, err := ParseDaily(validDailyFixture, []string{"USD", "CAD"})
	if err != nil {
		t.Fatalf("ParseDaily() error = %v", err)
	}

	if len(rates) != 1 {
		t.Fatalf("len(rates) = %d, want 1", len(rates))
	}

	if rates[0].CurrencyCode != "USD" {
		t.Fatalf("CurrencyCode = %q, want USD", rates[0].CurrencyCode)
	}
}
