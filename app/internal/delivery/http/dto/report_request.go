package dto

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

type ReportRequest struct {
	StartDate  time.Time
	EndDate    time.Time
	Currencies []string
}

type SyncRequest struct {
	StartDate time.Time
	EndDate   *time.Time
}

func ParseReportRequest(values url.Values) (ReportRequest, error) {
	startDate, err := ParseDateParam(values, "startDate")
	if err != nil {
		return ReportRequest{}, err
	}

	endDate, err := ParseDateParam(values, "endDate")
	if err != nil {
		return ReportRequest{}, err
	}

	currencies := parseCurrencies(values.Get("currencies"))
	if len(currencies) == 0 {
		return ReportRequest{}, fmt.Errorf("currencies must not be empty")
	}

	return ReportRequest{
		StartDate:  startDate,
		EndDate:    endDate,
		Currencies: currencies,
	}, nil
}

func ParseSyncRequest(values url.Values) (SyncRequest, error) {
	startDate, err := ParseDateParam(values, "startDate")
	if err != nil {
		return SyncRequest{}, err
	}

	endDateRaw := strings.TrimSpace(values.Get("endDate"))
	if endDateRaw == "" {
		return SyncRequest{StartDate: startDate}, nil
	}

	endDate, err := ParseDateParam(values, "endDate")
	if err != nil {
		return SyncRequest{}, err
	}

	return SyncRequest{
		StartDate: startDate,
		EndDate:   &endDate,
	}, nil
}

func ParseDateParam(values url.Values, key string) (time.Time, error) {
	value := strings.TrimSpace(values.Get(key))
	if value == "" {
		return time.Time{}, fmt.Errorf("%s must be provided", key)
	}

	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be in %s format", key, dateLayout)
	}

	return parsed.UTC(), nil
}

func parseCurrencies(raw string) []string {
	parts := strings.Split(raw, ",")
	currencies := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		code := strings.ToUpper(strings.TrimSpace(part))
		if code == "" {
			continue
		}

		if _, exists := seen[code]; exists {
			continue
		}

		seen[code] = struct{}{}
		currencies = append(currencies, code)
	}

	return currencies
}
