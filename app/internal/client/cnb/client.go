package cnb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const dailyDateLayout = "02.01.2006"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) (*Client, error) {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalizedBaseURL == "" {
		return nil, fmt.Errorf("cnb base url must not be empty")
	}

	if _, err := url.ParseRequestURI(normalizedBaseURL); err != nil {
		return nil, fmt.Errorf("parse cnb base url: %w", err)
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		baseURL:    normalizedBaseURL,
		httpClient: httpClient,
	}, nil
}

func (c *Client) FetchDaily(ctx context.Context, date time.Time) (string, error) {
	if c == nil {
		return "", fmt.Errorf("cnb client is nil")
	}

	if date.IsZero() {
		return "", fmt.Errorf("daily date must be set")
	}

	query := url.Values{}
	query.Set("date", date.Format(dailyDateLayout))

	return c.fetch(ctx, "daily.txt", query)
}

func (c *Client) FetchYear(ctx context.Context, year int) (string, error) {
	if c == nil {
		return "", fmt.Errorf("cnb client is nil")
	}

	if year <= 0 {
		return "", fmt.Errorf("year must be positive")
	}

	query := url.Values{}
	query.Set("year", strconv.Itoa(year))

	return c.fetch(ctx, "year.txt", query)
}

func (c *Client) fetch(ctx context.Context, resource string, query url.Values) (string, error) {
	requestURL := c.baseURL + "/" + resource
	if encodedQuery := query.Encode(); encodedQuery != "" {
		requestURL += "?" + encodedQuery
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("build cnb request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform cnb request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cnb request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read cnb response body: %w", err)
	}

	if len(body) == 0 {
		return "", fmt.Errorf("cnb response body is empty")
	}

	return string(body), nil
}
