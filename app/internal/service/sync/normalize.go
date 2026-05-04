package sync

import "sync-exchange-rate/internal/domain"

func NormalizeRate(rate *domain.Rate) error {
	return rate.Normalize()
}

func NormalizeRates(rates []domain.Rate) error {
	for index := range rates {
		if err := NormalizeRate(&rates[index]); err != nil {
			return err
		}
	}

	return nil
}
