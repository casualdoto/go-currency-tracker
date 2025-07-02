// Package currency provides functions for working with currency rates from various sources.
package currency

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Structures for parsing API response
type DailyRates struct {
	Date   string            `json:"Date"`
	Valute map[string]Valute `json:"Valute"`
}

type Valute struct {
	ID       string  `json:"ID"`
	NumCode  string  `json:"NumCode"`
	CharCode string  `json:"CharCode"`
	Nominal  int     `json:"Nominal"`
	Name     string  `json:"Name"`
	Value    float64 `json:"Value"`
	Previous float64 `json:"Previous"`
}

// Get rates from the CBR site for the current date
func GetCBRRates() (*DailyRates, error) {
	return GetCBRRatesByDate("")
}

// Get rates from the CBR site for the specified date
// If date is an empty string, returns rates for the current date
// Date format: YYYY-MM-DD (for example, "2023-05-15")
func GetCBRRatesByDate(date string) (*DailyRates, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := "https://www.cbr-xml-daily.ru/daily_json.js"

	// If date is specified, form URL for archive
	if date != "" {
		// Convert date format from YYYY-MM-DD to format for API (YYYY/MM/DD)
		parsedDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
		}

		// For archive data use different URL format
		// In cbr-xml-daily.ru API archive data is available by URL like:
		// https://www.cbr-xml-daily.ru/archive/YYYY/MM/DD/daily_json.js
		formattedDate := parsedDate.Format("2006/01/02")
		url = fmt.Sprintf("https://www.cbr-xml-daily.ru/archive/%s/daily_json.js", formattedDate)
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CBR rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If archive data not found, try to get data for previous working day
		if date != "" && resp.StatusCode == http.StatusNotFound {
			// Try to get data for current date
			return GetCBRRates()
		}
		return nil, fmt.Errorf("failed to fetch CBR rates, status code: %d", resp.StatusCode)
	}

	var rates DailyRates
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &rates, nil
}

// Get rate of specific currency
// code - currency code in ISO 4217 format (for example, USD, EUR)
// date - date in YYYY-MM-DD format, if empty string - current date
func GetCurrencyRate(code string, date string) (*Valute, error) {
	if code == "" {
		return nil, fmt.Errorf("currency code cannot be empty")
	}

	rates, err := GetCBRRatesByDate(date)
	if err != nil {
		return nil, err
	}

	valute, ok := rates.Valute[code]
	if !ok {
		return nil, fmt.Errorf("currency with code %s not found", code)
	}

	return &valute, nil
}
