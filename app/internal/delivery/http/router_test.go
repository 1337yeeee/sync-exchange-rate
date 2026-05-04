package http

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sync-exchange-rate/internal/delivery/http/handler"
	reportservice "sync-exchange-rate/internal/service/report"
	syncservice "sync-exchange-rate/internal/service/sync"
)

type fakeReportService struct {
	report reportservice.Report
	err    error
}

func (s *fakeReportService) BuildReport(_ context.Context, startDate, endDate time.Time, currencies []string) (reportservice.Report, error) {
	if s.err != nil {
		return reportservice.Report{}, s.err
	}

	return s.report, nil
}

type fakeSyncService struct {
	dateResult   syncservice.Result
	periodResult syncservice.Result
	err          error
}

func (s *fakeSyncService) SyncDate(_ context.Context, date time.Time) (syncservice.Result, error) {
	if s.err != nil {
		return syncservice.Result{}, s.err
	}

	return s.dateResult, nil
}

func (s *fakeSyncService) SyncPeriod(_ context.Context, startDate, endDate time.Time) (syncservice.Result, error) {
	if s.err != nil {
		return syncservice.Result{}, s.err
	}

	return s.periodResult, nil
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{}, &fakeSyncService{})
	request := httptest.NewRequest(stdhttp.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestReportHandlerReturnsValidJSON(t *testing.T) {
	t.Parallel()

	minRate := 24.9
	maxRate := 25.4
	avgRate := 25.15
	reportSvc := &fakeReportService{
		report: reportservice.Report{
			StartDate: time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC),
			Currencies: []reportservice.CurrencyReport{
				{
					CurrencyCode: "USD",
					MinRate:      &minRate,
					MaxRate:      &maxRate,
					AvgRate:      &avgRate,
					Observations: 2,
				},
			},
		},
	}

	router := newTestRouter(t, reportSvc, &fakeSyncService{})
	request := httptest.NewRequest(stdhttp.MethodGet, "/reports/rates?startDate=2024-03-01&endDate=2024-03-02&currencies=USD", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
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

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.StartDate != "2024-03-01" || response.EndDate != "2024-03-02" {
		t.Fatalf("unexpected dates in response: %+v", response)
	}

	if len(response.Currencies) != 1 {
		t.Fatalf("len(currencies) = %d, want 1", len(response.Currencies))
	}

	if response.Currencies[0].CurrencyCode != "USD" {
		t.Fatalf("CurrencyCode = %q, want USD", response.Currencies[0].CurrencyCode)
	}

	if response.Currencies[0].AvgRate == nil || *response.Currencies[0].AvgRate != 25.15 {
		t.Fatalf("AvgRate = %v, want 25.15", response.Currencies[0].AvgRate)
	}
}

func TestReportHandlerRejectsInvalidDates(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{}, &fakeSyncService{})
	request := httptest.NewRequest(stdhttp.MethodGet, "/reports/rates?startDate=2024/03/01&endDate=2024-03-02&currencies=USD", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestReportHandlerRejectsEmptyCurrencies(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{}, &fakeSyncService{})
	request := httptest.NewRequest(stdhttp.MethodGet, "/reports/rates?startDate=2024-03-01&endDate=2024-03-02&currencies=", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestReportHandlerReturnsServiceError(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{err: fmt.Errorf("report failed")}, &fakeSyncService{})
	request := httptest.NewRequest(stdhttp.MethodGet, "/reports/rates?startDate=2024-03-01&endDate=2024-03-02&currencies=USD", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}

func TestSyncHandlerSyncsPeriod(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{}, &fakeSyncService{
		periodResult: syncservice.Result{
			SavedCount:   4,
			SkippedCount: 1,
			Errors:       []string{"fetch year 2020: status 404"},
		},
	})
	request := httptest.NewRequest(stdhttp.MethodPost, "/sync?startDate=2024-03-01&endDate=2024-03-02", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var response struct {
		SavedCount   int      `json:"savedCount"`
		SkippedCount int      `json:"skippedCount"`
		Errors       []string `json:"errors"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.SavedCount != 4 || response.SkippedCount != 1 {
		t.Fatalf("unexpected sync response: %+v", response)
	}
}

func TestSyncHandlerRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{}, &fakeSyncService{})
	request := httptest.NewRequest(stdhttp.MethodPost, "/sync?startDate=2024-03-XX", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestSyncHandlerReturnsServiceError(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, &fakeReportService{}, &fakeSyncService{err: fmt.Errorf("sync failed")})
	request := httptest.NewRequest(stdhttp.MethodPost, "/sync?startDate=2024-03-01", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}

func newTestRouter(t *testing.T, reportService *fakeReportService, syncService *fakeSyncService) stdhttp.Handler {
	t.Helper()

	return NewRouter(
		handler.NewHealthHandler(),
		handler.NewSyncHandler(syncService),
		handler.NewReportHandler(reportService),
	)
}
