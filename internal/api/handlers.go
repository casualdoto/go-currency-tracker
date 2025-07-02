// Package api provides HTTP request handlers and API route setup.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
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
	dateStr := r.URL.Query().Get("date")

	var date time.Time
	var err error

	// Parse date or use current date
	if dateStr == "" {
		date = time.Now().Truncate(24 * time.Hour)
	} else {
		// Parse date from string
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Invalid date format. Use YYYY-MM-DD",
			})
			return
		}
	}

	// Check if we have a database connection in the context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if ok && db != nil {
		// Try to get rates from database first
		rates, err := db.GetCurrencyRatesByDate(date)
		if err == nil && len(rates) > 0 {
			// Convert database rates to response format
			response := APIResponse{
				Success: true,
				Data:    formatDBRatesToValuteMap(rates),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// If we don't have data in DB or there was an error, get from CBR API
	cbrDateStr := ""
	if dateStr != "" {
		cbrDateStr = dateStr
	}

	// Get currency rates from CBR
	rates, err := currency.GetCBRRatesByDate(cbrDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// If we have a database connection, save the rates
	if ok && db != nil {
		// Convert API rates to database format
		var dbRates []storage.CurrencyRate
		for code, valute := range rates.Valute {
			dbRates = append(dbRates, storage.CurrencyRate{
				Date:         date,
				CurrencyCode: code,
				CurrencyName: valute.Name,
				Nominal:      valute.Nominal,
				Value:        valute.Value,
				Previous:     valute.Previous,
			})
		}

		// Save to database in background to not block the response
		go func(dbRates []storage.CurrencyRate) {
			if err := db.SaveCurrencyRates(dbRates); err != nil {
				// Just log the error, don't affect the response
				fmt.Printf("Failed to save currency rates to database: %v\n", err)
			}
		}(dbRates)
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    rates.Valute,
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
	dateStr := r.URL.Query().Get("date")

	var date time.Time
	var err error

	// Parse date or use current date
	if dateStr == "" {
		date = time.Now().Truncate(24 * time.Hour)
	} else {
		// Parse date from string
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Invalid date format. Use YYYY-MM-DD",
			})
			return
		}
	}

	// Check if we have a database connection in the context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if ok && db != nil {
		// Try to get rate from database first
		rate, err := db.GetCurrencyRate(currencyCode, date)
		if err == nil {
			// Convert database rate to response format
			valuteRate := currency.Valute{
				ID:       rate.CurrencyCode,
				NumCode:  "",
				CharCode: rate.CurrencyCode,
				Nominal:  rate.Nominal,
				Name:     rate.CurrencyName,
				Value:    rate.Value,
				Previous: rate.Previous,
			}

			response := APIResponse{
				Success: true,
				Data:    valuteRate,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// If we don't have data in DB or there was an error, get from CBR API
	cbrDateStr := ""
	if dateStr != "" {
		cbrDateStr = dateStr
	}

	// Get currency rate from CBR API
	rate, err := currency.GetCurrencyRate(currencyCode, cbrDateStr)
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

	// If we have a database connection, save the rate
	if ok && db != nil {
		// Create a database rate record
		dbRate := storage.CurrencyRate{
			Date:         date,
			CurrencyCode: rate.CharCode,
			CurrencyName: rate.Name,
			Nominal:      rate.Nominal,
			Value:        rate.Value,
			Previous:     rate.Previous,
		}

		// Save to database in background to not block the response
		go func(dbRate storage.CurrencyRate) {
			if err := db.SaveCurrencyRates([]storage.CurrencyRate{dbRate}); err != nil {
				// Just log the error, don't affect the response
				fmt.Printf("Failed to save currency rate to database: %v\n", err)
			}
		}(dbRate)
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    rate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCurrencyHistoryHandler handles requests for getting historical currency rates.
// Requires query parameter code (currency code, e.g. USD).
// Requires query parameter days (number of days to look back).
func GetCurrencyHistoryHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get days parameter from request
	daysStr := r.URL.Query().Get("days")
	if daysStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Days parameter not specified (parameter days)",
		})
		return
	}

	// Parse days parameter
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid days parameter, must be a positive integer",
		})
		return
	}

	// Limit days to 365 to prevent excessive requests
	if days > 365 {
		days = 365
	}

	// Get current date
	today := time.Now().Truncate(24 * time.Hour)

	// Array to store historical rates
	history := []map[string]interface{}{}

	// Check if we have a database connection in the context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)

	// Collect rates for each day
	for i := 0; i < days; i++ {
		date := today.AddDate(0, 0, -i)

		var rate *currency.Valute
		var err error

		// Try to get from DB first if available
		if ok && db != nil {
			dbRate, dbErr := db.GetCurrencyRate(currencyCode, date)
			if dbErr == nil {
				// Convert DB rate to Valute format
				rate = &currency.Valute{
					ID:       dbRate.CurrencyCode,
					NumCode:  "",
					CharCode: dbRate.CurrencyCode,
					Nominal:  dbRate.Nominal,
					Name:     dbRate.CurrencyName,
					Value:    dbRate.Value,
					Previous: dbRate.Previous,
				}
			}
		}

		// If not found in DB, get from CBR API
		if rate == nil {
			dateStr := date.Format("2006-01-02")
			rate, err = currency.GetCurrencyRate(currencyCode, dateStr)
			if err != nil {
				// Skip this date if there's an error
				continue
			}

			// Save to DB in background if we have a connection
			if ok && db != nil {
				go func(date time.Time, rate *currency.Valute) {
					dbRate := storage.CurrencyRate{
						Date:         date,
						CurrencyCode: rate.CharCode,
						CurrencyName: rate.Name,
						Nominal:      rate.Nominal,
						Value:        rate.Value,
						Previous:     rate.Previous,
					}
					if err := db.SaveCurrencyRates([]storage.CurrencyRate{dbRate}); err != nil {
						fmt.Printf("Failed to save currency rate to database: %v\n", err)
					}
				}(date, rate)
			}
		}

		// Add rate to history
		history = append(history, map[string]interface{}{
			"date":     date.Format("2006-01-02"),
			"code":     rate.CharCode,
			"name":     rate.Name,
			"nominal":  rate.Nominal,
			"value":    rate.Value,
			"previous": rate.Previous,
		})
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    history,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
