package cnb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchDailySuccess(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotDate string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotDate = r.URL.Query().Get("date")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("daily rates"))
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL, &http.Client{Timeout: time.Second})

	body, err := client.FetchDaily(context.Background(), time.Date(2024, time.March, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchDaily() error = %v", err)
	}

	if body != "daily rates" {
		t.Fatalf("FetchDaily() body = %q, want %q", body, "daily rates")
	}

	if gotPath != "/daily.txt" {
		t.Fatalf("request path = %q, want %q", gotPath, "/daily.txt")
	}

	if gotDate != "05.03.2024" {
		t.Fatalf("date query = %q, want %q", gotDate, "05.03.2024")
	}
}

func TestFetchYearSuccess(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotYear string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotYear = r.URL.Query().Get("year")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("year rates"))
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL+"/", &http.Client{Timeout: time.Second})

	body, err := client.FetchYear(context.Background(), 2024)
	if err != nil {
		t.Fatalf("FetchYear() error = %v", err)
	}

	if body != "year rates" {
		t.Fatalf("FetchYear() body = %q, want %q", body, "year rates")
	}

	if gotPath != "/year.txt" {
		t.Fatalf("request path = %q, want %q", gotPath, "/year.txt")
	}

	if gotYear != "2024" {
		t.Fatalf("year query = %q, want %q", gotYear, "2024")
	}
}

func TestFetchDailyReturnsStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL, &http.Client{Timeout: time.Second})

	_, err := client.FetchDaily(context.Background(), time.Date(2024, time.March, 5, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("FetchDaily() error = %v, want status 404", err)
	}
}

func TestFetchYearReturnsStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL, &http.Client{Timeout: time.Second})

	_, err := client.FetchYear(context.Background(), 2024)
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("FetchYear() error = %v, want status 500", err)
	}
}

func TestFetchDailyTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("late daily rates"))
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL, &http.Client{Timeout: 20 * time.Millisecond})

	_, err := client.FetchDaily(context.Background(), time.Date(2024, time.March, 5, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "perform cnb request") {
		t.Fatalf("FetchDaily() error = %v, want timeout request error", err)
	}
}

func TestFetchDailyReturnsBodyReadError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("response writer does not support hijacking")
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("Hijack() error = %v", err)
		}
		defer conn.Close()

		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 20\r\nContent-Type: text/plain\r\n\r\nshort"))
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL, &http.Client{Timeout: time.Second})

	_, err := client.FetchDaily(context.Background(), time.Date(2024, time.March, 5, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "read cnb response body") {
		t.Fatalf("FetchDaily() error = %v, want body read error", err)
	}
}

func TestFetchDailyRejectsZeroDate(t *testing.T) {
	t.Parallel()

	client := mustNewClient(t, "https://example.test/cnb", &http.Client{Timeout: time.Second})

	_, err := client.FetchDaily(context.Background(), time.Time{})
	if err == nil || !strings.Contains(err.Error(), "daily date must be set") {
		t.Fatalf("FetchDaily() error = %v, want zero date validation error", err)
	}
}

func TestFetchYearRejectsInvalidYear(t *testing.T) {
	t.Parallel()

	client := mustNewClient(t, "https://example.test/cnb", &http.Client{Timeout: time.Second})

	_, err := client.FetchYear(context.Background(), 0)
	if err == nil || !strings.Contains(err.Error(), "year must be positive") {
		t.Fatalf("FetchYear() error = %v, want year validation error", err)
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	t.Parallel()

	_, err := NewClient("://bad-url", nil)
	if err == nil {
		t.Fatal("NewClient() error = nil, want parse error")
	}
}

func TestFetchDailyRejectsEmptyBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL, &http.Client{Timeout: time.Second})

	_, err := client.FetchDaily(context.Background(), time.Date(2024, time.March, 5, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "response body is empty") {
		t.Fatalf("FetchDaily() error = %v, want empty body error", err)
	}
}

func mustNewClient(t *testing.T, baseURL string, httpClient *http.Client) *Client {
	t.Helper()

	client, err := NewClient(baseURL, httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return client
}
