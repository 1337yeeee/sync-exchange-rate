package cnb

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var cnbDateLayouts = []string{
	"02.01.2006",
	"2.1.2006",
	"02 Jan 2006",
	"2 Jan 2006",
	"02.Jan.2006",
	"2.Jan.2006",
	"2006-01-02",
	"2006Jan02",
}

func parseCNBDate(value string) (time.Time, error) {
	normalized := strings.TrimSpace(value)
	if hashIndex := strings.Index(normalized, "#"); hashIndex >= 0 {
		normalized = strings.TrimSpace(normalized[:hashIndex])
	}

	for _, layout := range cnbDateLayouts {
		parsed, err := time.Parse(layout, normalized)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("parse cnb date %q", value)
}

func parseCNBAmount(value string) (int, error) {
	amount, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}

	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}

	return amount, nil
}

func parseCNBRate(value string) (float64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	if normalized == "" {
		return 0, fmt.Errorf("rate must not be empty")
	}

	return strconv.ParseFloat(normalized, 64)
}

func currencyFilter(currencies []string) map[string]struct{} {
	if len(currencies) == 0 {
		return nil
	}

	filter := make(map[string]struct{}, len(currencies))
	for _, currency := range currencies {
		code := normalizeCurrencyCode(currency)
		if code == "" {
			continue
		}
		filter[code] = struct{}{}
	}

	if len(filter) == 0 {
		return nil
	}

	return filter
}

func shouldKeepCurrency(filter map[string]struct{}, code string) bool {
	if len(filter) == 0 {
		return true
	}

	_, ok := filter[normalizeCurrencyCode(code)]
	return ok
}

func normalizeCurrencyCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func splitNonEmptyLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	lines := strings.Split(raw, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}

	return result
}

func inDateRange(date, start, end time.Time) bool {
	if !start.IsZero() && date.Before(start.UTC()) {
		return false
	}

	if !end.IsZero() && date.After(end.UTC()) {
		return false
	}

	return true
}
