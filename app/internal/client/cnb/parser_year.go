package cnb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"sync-exchange-rate/internal/domain"
)

type yearColumn struct {
	Amount int
	Code   string
}

func ParseYear(raw string, currencies []string, startDate, endDate time.Time) ([]domain.Rate, error) {
	lines := splitNonEmptyLines(raw)
	if len(lines) == 0 {
		return nil, fmt.Errorf("year rates response is empty")
	}

	filter := currencyFilter(currencies)
	var columns []yearColumn
	rates := make([]domain.Rate, 0, len(lines))

	for _, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) == 0 {
			continue
		}

		if strings.TrimSpace(fields[0]) == "Date" {
			parsedColumns, err := parseYearHeader(fields)
			if err != nil {
				return nil, err
			}
			columns = parsedColumns
			continue
		}

		if len(columns) == 0 {
			return nil, fmt.Errorf("year rates header is missing")
		}

		tradingDate, err := parseCNBDate(fields[0])
		if err != nil {
			continue
		}

		if !inDateRange(tradingDate, startDate, endDate) {
			continue
		}

		for index, column := range columns {
			fieldIndex := index + 1
			if fieldIndex >= len(fields) {
				continue
			}

			if !shouldKeepCurrency(filter, column.Code) {
				continue
			}

			rateValue, err := parseCNBRate(fields[fieldIndex])
			if err != nil {
				continue
			}

			rate := domain.Rate{
				TradingDate:  tradingDate,
				Country:      "",
				CurrencyName: column.Code,
				Amount:       column.Amount,
				CurrencyCode: column.Code,
				Rate:         rateValue,
			}

			if err := rate.Normalize(); err != nil {
				continue
			}

			rates = append(rates, rate)
		}
	}

	return rates, nil
}

func parseYearHeader(fields []string) ([]yearColumn, error) {
	if len(fields) < 2 {
		return nil, fmt.Errorf("year rates header does not contain currencies")
	}

	columns := make([]yearColumn, 0, len(fields)-1)
	for _, field := range fields[1:] {
		parts := strings.Fields(strings.TrimSpace(field))
		if len(parts) != 2 {
			continue
		}

		amount, err := strconv.Atoi(parts[0])
		if err != nil || amount <= 0 {
			continue
		}

		code := normalizeCurrencyCode(parts[1])
		if code == "" {
			continue
		}

		columns = append(columns, yearColumn{
			Amount: amount,
			Code:   code,
		})
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("year rates header does not contain valid currencies")
	}

	return columns, nil
}
