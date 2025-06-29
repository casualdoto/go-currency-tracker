// Package api provides HTTP request handlers and API route setup.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency"
)

// APIResponse represents standard API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

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

// CBRRatesHandler handles requests for getting CBR currency rates.
// Supports optional query parameter date in DD/MM/YYYY format.
// If date parameter is not specified, returns rates for the current date.
func CBRRatesHandler(w http.ResponseWriter, r *http.Request) {
	// Get date parameter from request (optional)
	date := r.URL.Query().Get("date")

	// Get currency rates from CBR
	rates, err := currency.GetCBRRatesByDate(date)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    rates.Valute,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CBRCurrencyHandler handles requests for getting a specific CBR currency rate.
// Requires query parameter code (currency code, e.g. USD).
// Supports optional query parameter date in DD/MM/YYYY format.
// If date parameter is not specified, returns rate for the current date.
func CBRCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	// Get currency code from request
	currencyCode := r.URL.Query().Get("code")
	if currencyCode == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Currency code not specified (parameter code)",
		})
		return
	}

	// Get date parameter from request (optional)
	date := r.URL.Query().Get("date")

	// Get currency rate
	rate, err := currency.GetCurrencyRate(currencyCode, date)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "currency with code "+currencyCode+" not found" {
			statusCode = http.StatusNotFound
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    rate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
