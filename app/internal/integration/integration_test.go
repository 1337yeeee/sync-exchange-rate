package integration

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	deliveryhttp "sync-exchange-rate/internal/delivery/http"
	"sync-exchange-rate/internal/delivery/http/handler"
	"sync-exchange-rate/internal/domain"
	"sync-exchange-rate/internal/repository"
	raterepository "sync-exchange-rate/internal/repository/postgres"
	reportservice "sync-exchange-rate/internal/service/report"
	syncservice "sync-exchange-rate/internal/service/sync"
	"sync-exchange-rate/internal/storage/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const integrationYearFixture = `Date|1 EUR|1 USD
26.07.2019|25.540|22.932
28.07.2019|25.640|23.132
`

type fakeCNBClient struct {
	dailyResponses map[string]string
	yearResponses  map[int]string
}

func (c *fakeCNBClient) FetchDaily(_ context.Context, date time.Time) (string, error) {
	return c.dailyResponses[date.UTC().Format("2006-01-02")], nil
}

func (c *fakeCNBClient) FetchYear(_ context.Context, year int) (string, error) {
	return c.yearResponses[year], nil
}

func TestIntegrationSyncPipelineToReport(t *testing.T) {
	db := openIntegrationDB(t)
	repo := raterepository.NewRateRepository(db)
	client := &fakeCNBClient{
		dailyResponses: map[string]string{},
		yearResponses: map[int]string{
			2019: integrationYearFixture,
		},
	}

	syncSvc := mustNewSyncService(t, client, repo)
	reportSvc := mustNewReportService(t, repo)

	result, err := syncSvc.SyncPeriod(
		context.Background(),
		time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC),
		time.Date(2019, time.July, 28, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("SyncPeriod() error = %v", err)
	}

	if result.SavedCount != 4 {
		t.Fatalf("SavedCount = %d, want 4", result.SavedCount)
	}

	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %v, want none", result.Errors)
	}

	report, err := reportSvc.BuildReport(
		context.Background(),
		time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC),
		time.Date(2019, time.July, 28, 0, 0, 0, 0, time.UTC),
		[]string{"EUR", "USD"},
	)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}

	if len(report.Currencies) != 2 {
		t.Fatalf("len(report.Currencies) = %d, want 2", len(report.Currencies))
	}

	eur := report.Currencies[0]
	usd := report.Currencies[1]

	assertFloatPtr(t, eur.MinRate, 25.54, "EUR MinRate")
	assertFloatPtr(t, eur.MaxRate, 25.64, "EUR MaxRate")
	assertFloatPtr(t, eur.AvgRate, 25.59, "EUR AvgRate")
	if eur.Observations != 2 {
		t.Fatalf("EUR Observations = %d, want 2", eur.Observations)
	}

	assertFloatPtr(t, usd.MinRate, 22.932, "USD MinRate")
	assertFloatPtr(t, usd.MaxRate, 23.132, "USD MaxRate")
	assertFloatPtr(t, usd.AvgRate, 23.032, "USD AvgRate")
	if usd.Observations != 2 {
		t.Fatalf("USD Observations = %d, want 2", usd.Observations)
	}
}

func TestIntegrationRepeatedSyncDoesNotCreateDuplicates(t *testing.T) {
	db := openIntegrationDB(t)
	repo := raterepository.NewRateRepository(db)
	client := &fakeCNBClient{
		dailyResponses: map[string]string{},
		yearResponses: map[int]string{
			2019: integrationYearFixture,
		},
	}

	syncSvc := mustNewSyncService(t, client, repo)
	startDate := time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2019, time.July, 28, 0, 0, 0, 0, time.UTC)

	firstResult, err := syncSvc.SyncPeriod(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("first SyncPeriod() error = %v", err)
	}

	secondResult, err := syncSvc.SyncPeriod(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("second SyncPeriod() error = %v", err)
	}

	if firstResult.SavedCount != 4 {
		t.Fatalf("first SavedCount = %d, want 4", firstResult.SavedCount)
	}

	if secondResult.SavedCount != 0 || secondResult.SkippedCount != 4 {
		t.Fatalf("second result = %+v, want saved=0 skipped=4", secondResult)
	}

	var count int64
	if err := db.Model(&domain.Rate{}).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 4 {
		t.Fatalf("row count = %d, want 4", count)
	}
}

func TestIntegrationHTTPSyncThenReport(t *testing.T) {
	db := openIntegrationDB(t)
	repo := raterepository.NewRateRepository(db)
	client := &fakeCNBClient{
		dailyResponses: map[string]string{},
		yearResponses: map[int]string{
			2019: integrationYearFixture,
		},
	}

	syncSvc := mustNewSyncService(t, client, repo)
	reportSvc := mustNewReportService(t, repo)
	router := deliveryhttp.NewRouter(
		handler.NewHealthHandler(),
		handler.NewSyncHandler(syncSvc),
		handler.NewReportHandler(reportSvc),
	)

	syncRequest := httptest.NewRequest(stdhttp.MethodPost, "/sync?startDate=2019-07-26&endDate=2019-07-28", nil)
	syncRecorder := httptest.NewRecorder()
	router.ServeHTTP(syncRecorder, syncRequest)

	if syncRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("sync status = %d, want 200", syncRecorder.Code)
	}

	reportRequest := httptest.NewRequest(stdhttp.MethodGet, "/reports/rates?startDate=2019-07-26&endDate=2019-07-28&currencies=EUR,USD", nil)
	reportRecorder := httptest.NewRecorder()
	router.ServeHTTP(reportRecorder, reportRequest)

	if reportRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("report status = %d, want 200", reportRecorder.Code)
	}

	var response struct {
		StartDate  string `json:"startDate"`
		EndDate    string `json:"endDate"`
		Currencies []struct {
			CurrencyCode string   `json:"currencyCode"`
			MinRate      *float64 `json:"minRate"`
			MaxRate      *float64 `json:"maxRate"`
			AvgRate      *float64 `json:"avgRate"`
			Observations int      `json:"observations"`
		} `json:"currencies"`
	}

	if err := json.Unmarshal(reportRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.StartDate != "2019-07-26" || response.EndDate != "2019-07-28" {
		t.Fatalf("unexpected response dates: %+v", response)
	}

	if len(response.Currencies) != 2 {
		t.Fatalf("len(currencies) = %d, want 2", len(response.Currencies))
	}

	if response.Currencies[0].CurrencyCode != "EUR" || response.Currencies[1].CurrencyCode != "USD" {
		t.Fatalf("unexpected currency order: %+v", response.Currencies)
	}

	if response.Currencies[0].Observations != 2 || response.Currencies[1].Observations != 2 {
		t.Fatalf("unexpected observations: %+v", response.Currencies)
	}
}

func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	return db
}

func mustNewSyncService(t *testing.T, client *fakeCNBClient, repo repository.RateRepository) *syncservice.Service {
	t.Helper()

	svc, err := syncservice.NewService(client, repo, []string{"EUR", "USD"})
	if err != nil {
		t.Fatalf("syncservice.NewService() error = %v", err)
	}

	return svc
}

func mustNewReportService(t *testing.T, repo repository.RateRepository) *reportservice.Service {
	t.Helper()

	svc, err := reportservice.NewService(repo)
	if err != nil {
		t.Fatalf("reportservice.NewService() error = %v", err)
	}

	return svc
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
