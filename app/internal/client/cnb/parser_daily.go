package cnb

import (
	"fmt"
	"strings"

	"sync-exchange-rate/internal/domain"
)

const dailyHeader = "Country|Currency|Amount|Code|Rate"

func ParseDaily(raw string, currencies []string) ([]domain.Rate, error) {
	lines := splitNonEmptyLines(raw)
	if len(lines) == 0 {
		return nil, fmt.Errorf("daily rates response is empty")
	}

	tradingDate, err := parseCNBDate(lines[0])
	if err != nil {
		return nil, fmt.Errorf("parse daily trading date: %w", err)
	}

	filter := currencyFilter(currencies)
	rates := make([]domain.Rate, 0, len(lines))

	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == dailyHeader {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) != 5 {
			continue
		}

		code := normalizeCurrencyCode(fields[3])
		if !shouldKeepCurrency(filter, code) {
			continue
		}

		amount, err := parseCNBAmount(fields[2])
		if err != nil {
			continue
		}

		rateValue, err := parseCNBRate(fields[4])
		if err != nil {
			continue
		}

		rate := domain.Rate{
			TradingDate:  tradingDate,
			Country:      strings.TrimSpace(fields[0]),
			CurrencyName: strings.TrimSpace(fields[1]),
			Amount:       amount,
			CurrencyCode: code,
			Rate:         rateValue,
		}

		if err := rate.Normalize(); err != nil {
			continue
		}

		rates = append(rates, rate)
	}

	return rates, nil
}
