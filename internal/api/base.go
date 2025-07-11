// Package api provides HTTP request handlers and API route setup.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	currency "github.com/casualdoto/go-currency-tracker/internal/currency/cbr"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
)

// CORSMiddleware adds CORS headers to support cross-domain requests.
// Allows requests from any origin and supports various HTTP methods.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Pass request to the next handler
		next.ServeHTTP(w, r)
	})
}

// PingHandler handles requests to check service availability.
// Returns a simple "pong" response to confirm API is working.
func PingHandler(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Data:    "pong",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// InfoHandler returns information about the service.
// Includes name, version, and service start time.
func InfoHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]string{
		"name":        "Go Currency Tracker API",
		"version":     "1.0.0",
		"description": "API for tracking currency rates",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	response := APIResponse{
		Success: true,
		Data:    info,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper function to convert database rates to the same format as CBR API
func formatDBRatesToValuteMap(rates []storage.CurrencyRate) map[string]currency.Valute {
	valute := make(map[string]currency.Valute)
	for _, rate := range rates {
		valute[rate.CurrencyCode] = currency.Valute{
			ID:       rate.CurrencyCode,
			NumCode:  "",
			CharCode: rate.CurrencyCode,
			Nominal:  rate.Nominal,
			Name:     rate.CurrencyName,
			Value:    rate.Value,
			Previous: rate.Previous,
		}
	}
	return valute
}

// Helper function to convert storage.CryptoRate to HistoricalCryptoRate
func convertCryptoRateToHistorical(rate storage.CryptoRate) HistoricalCryptoRate {
	return HistoricalCryptoRate{
		Timestamp: rate.Timestamp.Unix() * 1000, // Convert to milliseconds for JavaScript
		Date:      rate.Timestamp.Format("2006-01-02 15:04:05"),
		Open:      rate.Open,
		High:      rate.High,
		Low:       rate.Low,
		Close:     rate.Close,
		Volume:    rate.Volume,
	}
}
