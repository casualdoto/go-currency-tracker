package currency

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casualdoto/go-currency-tracker/internal/config"
)

// Mock server for testing CBR API
func setupMockCBRServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request path
		if r.URL.Path == "/daily_json.js" {
			// Return test data for current day
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"Date": "2023-06-29T11:30:00+03:00",
				"PreviousDate": "2023-06-28T11:30:00+03:00",
				"PreviousURL": "\/\/www.cbr-xml-daily.ru\/archive\/2023\/06\/28\/daily_json.js",
				"Timestamp": "2023-06-29T11:00:00+03:00",
				"Valute": {
					"USD": {
						"ID": "R01235",
						"NumCode": "840",
						"CharCode": "USD",
						"Nominal": 1,
						"Name": "Доллар США",
						"Value": 87.0341,
						"Previous": 87.1992
					},
					"EUR": {
						"ID": "R01239",
						"NumCode": "978",
						"CharCode": "EUR",
						"Nominal": 1,
						"Name": "Евро",
						"Value": 94.7092,
						"Previous": 95.0489
					}
				}
			}`))
			return
		}

		// For archive data
		if r.URL.Path == "/archive/2023/06/28/daily_json.js" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"Date": "2023-06-28T11:30:00+03:00",
				"PreviousDate": "2023-06-27T11:30:00+03:00",
				"PreviousURL": "\/\/www.cbr-xml-daily.ru\/archive\/2023\/06\/27\/daily_json.js",
				"Timestamp": "2023-06-28T11:00:00+03:00",
				"Valute": {
					"USD": {
						"ID": "R01235",
						"NumCode": "840",
						"CharCode": "USD",
						"Nominal": 1,
						"Name": "Доллар США",
						"Value": 85.0504,
						"Previous": 87.3332
					},
					"EUR": {
						"ID": "R01239",
						"NumCode": "978",
						"CharCode": "EUR",
						"Nominal": 1,
						"Name": "Евро",
						"Value": 93.1373,
						"Previous": 95.2346
					}
				}
			}`))
			return
		}

		// For non-existent dates
		w.WriteHeader(http.StatusNotFound)
	}))
}

// Testing getting CBR rates for current date
func TestGetCBRRates(t *testing.T) {
	// Create mock server
	server := setupMockCBRServer()
	defer server.Close()

	config.SetCBRBaseURLForTesting(server.URL)

	// Run test
	rates, err := GetCBRRates()
	if err != nil {
		t.Fatalf("Error getting currency rates: %v", err)
	}

	// Check for expected currencies
	if _, ok := rates.Valute["USD"]; !ok {
		t.Error("Expected USD currency in response")
	}
	if _, ok := rates.Valute["EUR"]; !ok {
		t.Error("Expected EUR currency in response")
	}

	// Check rate values
	if rates.Valute["USD"].Value == 0 {
		t.Error("USD rate value should not be 0")
	}
	if rates.Valute["EUR"].Value == 0 {
		t.Error("EUR rate value should not be 0")
	}
}

// Testing getting CBR rates for specified date
func TestGetCBRRatesByDate(t *testing.T) {
	// Create mock server
	server := setupMockCBRServer()
	defer server.Close()

	config.SetCBRBaseURLForTesting(server.URL)

	// Run test
	rates, err := GetCBRRatesByDate("2023-06-28")
	if err != nil {
		t.Fatalf("Error getting currency rates for specified date: %v", err)
	}

	// Check for expected currencies
	if _, ok := rates.Valute["USD"]; !ok {
		t.Error("Expected USD currency in response")
	}
	if _, ok := rates.Valute["EUR"]; !ok {
		t.Error("Expected EUR currency in response")
	}

	// Check rate values
	if rates.Valute["USD"].Value != 85.0504 {
		t.Errorf("Expected USD rate value 85.0504, got %v", rates.Valute["USD"].Value)
	}
	if rates.Valute["EUR"].Value != 93.1373 {
		t.Errorf("Expected EUR rate value 93.1373, got %v", rates.Valute["EUR"].Value)
	}
}

// Testing getting specific currency rate
func TestGetCurrencyRate(t *testing.T) {
	// Create mock server
	server := setupMockCBRServer()
	defer server.Close()

	config.SetCBRBaseURLForTesting(server.URL)

	// Run test for USD
	usdRate, err := GetCurrencyRate("USD", "")
	if err != nil {
		t.Fatalf("Error getting USD rate: %v", err)
	}

	// Check values
	if usdRate.CharCode != "USD" {
		t.Errorf("Expected currency code USD, got %s", usdRate.CharCode)
	}
	if usdRate.Name != "Доллар США" {
		t.Errorf("Expected name 'Доллар США', got %s", usdRate.Name)
	}

	// Run test for non-existent currency
	_, err = GetCurrencyRate("XYZ", "")
	if err == nil {
		t.Error("Expected error when requesting non-existent currency")
	}
}
