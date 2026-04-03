// Package cbrbackfill fetches historical CBR daily_json from cbr-xml-daily.ru archive
// when PostgreSQL has no rows yet (same URL layout as monolith/internal/currency/cbr).
package cbrbackfill

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

// maxArchiveLookbackDays is how far back we walk when cbr-xml-daily has no file (weekends/holidays).
const maxArchiveLookbackDays = 14

// Client downloads daily_json.js for a calendar day.
type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type dailyResponse struct {
	Date   string          `json:"Date"`
	Valute map[string]unit `json:"Valute"`
}

type unit struct {
	CharCode string  `json:"CharCode"`
	NumCode  string  `json:"NumCode"`
	Nominal  int     `json:"Nominal"`
	Name     string  `json:"Name"`
	Value    float64 `json:"Value"`
	Previous float64 `json:"Previous"`
}

// FetchDay downloads archive JSON for the given calendar day (local date parts used for URL path).
func (c *Client) FetchDay(day time.Time) ([]storage.CurrencyRate, error) {
	if c == nil {
		return nil, fmt.Errorf("cbr backfill client is nil")
	}
	path := day.Format("2006/01/02")
	url := fmt.Sprintf("%s/archive/%s/daily_json.js", c.baseURL, path)

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cbr get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("cbr archive not found for %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cbr status %d for %s", resp.StatusCode, path)
	}

	var data dailyResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("cbr decode: %w", err)
	}

	rateDate, err := parseCBRRootDate(data.Date)
	if err != nil {
		return nil, fmt.Errorf("cbr date field: %w", err)
	}

	out := make([]storage.CurrencyRate, 0, len(data.Valute))
	for _, v := range data.Valute {
		if v.CharCode == "" {
			continue
		}
		out = append(out, storage.CurrencyRate{
			Date:         rateDate,
			CurrencyCode: v.CharCode,
			CurrencyName: v.Name,
			Nominal:      v.Nominal,
			Value:        v.Value,
			Previous:     v.Previous,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cbr empty valute map for %s", path)
	}
	return out, nil
}

// FetchDayWithFallback tries archive for the given UTC calendar day, then earlier days up to
// maxArchiveLookbackDays until a file exists (same approach as carrying last known CBR over non-trading days).
// Returns rates from the archive file, and sourceDay — the calendar day of the file actually used.
func (c *Client) FetchDayWithFallback(day time.Time) (rates []storage.CurrencyRate, sourceDay time.Time, err error) {
	if c == nil {
		return nil, time.Time{}, fmt.Errorf("cbr backfill client is nil")
	}
	target := calendarDateUTC(day)
	var lastErr error
	for i := 0; i <= maxArchiveLookbackDays; i++ {
		cand := target.AddDate(0, 0, -i)
		rates, err := c.FetchDay(cand)
		if err == nil {
			return rates, cand, nil
		}
		lastErr = err
	}
	return nil, time.Time{}, lastErr
}

func calendarDateUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func parseCBRRootDate(s string) (time.Time, error) {
	layouts := []string{
		"2006/01/02 15:04:05",
		"2006-01-02T15:04:05-07:00",
		time.RFC3339,
		"02.01.2006",
		"2006-01-02",
	}
	var lastErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			y, m, d := t.Date()
			return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("parse CBR date %q: %v", s, lastErr)
}
