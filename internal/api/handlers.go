// Package api provides HTTP request handlers and API route setup.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency/binance"
	currency "github.com/casualdoto/go-currency-tracker/internal/currency/cbr"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
	"github.com/xuri/excelize/v2"
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

	// If we have a database connection, save all rates for this day
	if ok && db != nil {
		// Get all rates for this date to save them all at once
		rates, err := currency.GetCBRRatesByDate(cbrDateStr)
		if err == nil {
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
		} else {
			// If we couldn't get all rates, at least save this one
			dbRate := storage.CurrencyRate{
				Date:         date,
				CurrencyCode: rate.CharCode,
				CurrencyName: rate.Name,
				Nominal:      rate.Nominal,
				Value:        rate.Value,
				Previous:     rate.Previous,
			}

			// Save to database in background
			go func(dbRate storage.CurrencyRate) {
				if err := db.SaveCurrencyRates([]storage.CurrencyRate{dbRate}); err != nil {
					fmt.Printf("Failed to save currency rate to database: %v\n", err)
				}
			}(dbRate)
		}
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

			// Save all rates for this day to DB if we have a connection
			if ok && db != nil {
				go func(dateStr string, date time.Time, currentRate *currency.Valute) {
					// Get all rates for this date
					rates, err := currency.GetCBRRatesByDate(dateStr)
					if err == nil {
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

						// Save to database
						if err := db.SaveCurrencyRates(dbRates); err != nil {
							fmt.Printf("Failed to save currency rates to database: %v\n", err)
						}
					} else {
						// If we couldn't get all rates, at least save this one
						dbRate := storage.CurrencyRate{
							Date:         date,
							CurrencyCode: currentRate.CharCode,
							CurrencyName: currentRate.Name,
							Nominal:      currentRate.Nominal,
							Value:        currentRate.Value,
							Previous:     currentRate.Previous,
						}
						if err := db.SaveCurrencyRates([]storage.CurrencyRate{dbRate}); err != nil {
							fmt.Printf("Failed to save currency rate to database: %v\n", err)
						}
					}
				}(dateStr, date, rate)
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

// GetCurrencyHistoryByDateRangeHandler handles requests for getting historical currency rates by date range.
// Requires query parameter code (currency code, e.g. USD).
// Requires query parameters start_date and end_date in YYYY-MM-DD format.
func GetCurrencyHistoryByDateRangeHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get date parameters from request
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if startDateStr == "" || endDateStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Both start_date and end_date parameters are required (format: YYYY-MM-DD)",
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Validate date range
	if startDate.After(endDate) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "start_date must be before or equal to end_date",
		})
		return
	}

	// Limit range to 365 days
	if endDate.Sub(startDate) > 365*24*time.Hour {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Date range cannot exceed 365 days",
		})
		return
	}

	// Check if we have a database connection in the context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if !ok || db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Database connection not available",
		})
		return
	}

	// Get rates from database for the date range
	dbRates, err := db.GetCurrencyRatesByDateRange(currencyCode, startDate, endDate)

	// Array to store historical rates
	history := []map[string]interface{}{}

	// If we have data from DB, use it
	if err == nil && len(dbRates) > 0 {
		for _, dbRate := range dbRates {
			history = append(history, map[string]interface{}{
				"date":     dbRate.Date.Format("2006-01-02"),
				"code":     dbRate.CurrencyCode,
				"name":     dbRate.CurrencyName,
				"nominal":  dbRate.Nominal,
				"value":    dbRate.Value,
				"previous": dbRate.Previous,
			})
		}
	} else {
		// If no data in DB or there was an error, get from CBR API
		// We'll iterate through each date in the range
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			dateStr := d.Format("2006-01-02")
			rate, err := currency.GetCurrencyRate(currencyCode, dateStr)

			if err != nil {
				// Skip this date if there's an error (might be weekend/holiday)
				continue
			}

			// Add rate to history
			history = append(history, map[string]interface{}{
				"date":     dateStr,
				"code":     rate.CharCode,
				"name":     rate.Name,
				"nominal":  rate.Nominal,
				"value":    rate.Value,
				"previous": rate.Previous,
			})

			// Save all rates for this day to DB
			go func(dateStr string, date time.Time, currentRate *currency.Valute) {
				// Get all rates for this date
				rates, err := currency.GetCBRRatesByDate(dateStr)
				if err == nil {
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

					// Save to database
					if err := db.SaveCurrencyRates(dbRates); err != nil {
						fmt.Printf("Failed to save currency rates to database: %v\n", err)
					}
				} else {
					// If we couldn't get all rates, at least save this one
					dbRate := storage.CurrencyRate{
						Date:         date,
						CurrencyCode: currentRate.CharCode,
						CurrencyName: currentRate.Name,
						Nominal:      currentRate.Nominal,
						Value:        currentRate.Value,
						Previous:     currentRate.Previous,
					}
					if err := db.SaveCurrencyRates([]storage.CurrencyRate{dbRate}); err != nil {
						fmt.Printf("Failed to save currency rate to database: %v\n", err)
					}
				}
			}(dateStr, d, rate)
		}
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    history,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExportCurrencyHistoryToExcelHandler handles requests for exporting historical currency rates by date range to Excel.
// Requires query parameter code (currency code, e.g. USD).
// Requires query parameters start_date and end_date in YYYY-MM-DD format.
func ExportCurrencyHistoryToExcelHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get date parameters from request
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if startDateStr == "" || endDateStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Both start_date and end_date parameters are required (format: YYYY-MM-DD)",
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Validate date range
	if startDate.After(endDate) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "start_date must be before or equal to end_date",
		})
		return
	}

	// Limit range to 365 days
	if endDate.Sub(startDate) > 365*24*time.Hour {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Date range cannot exceed 365 days",
		})
		return
	}

	// Check if we have a database connection in the context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if !ok || db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Database connection not available",
		})
		return
	}

	// Get rates from database for the date range
	dbRates, err := db.GetCurrencyRatesByDateRange(currencyCode, startDate, endDate)

	// Map to store historical rates by date for quick lookup
	ratesByDate := make(map[string]storage.CurrencyRate)

	// If we have data from DB, add it to the map
	if err == nil && len(dbRates) > 0 {
		for _, rate := range dbRates {
			dateStr := rate.Date.Format("2006-01-02")
			ratesByDate[dateStr] = rate
		}
	}

	// Array to store all historical rates in order
	history := []storage.CurrencyRate{}

	// Iterate through each date in the range to ensure we have data for all dates
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		// Check if we already have data for this date from DB
		if rate, exists := ratesByDate[dateStr]; exists {
			history = append(history, rate)
			continue
		}

		// If not in DB, try to get from API
		apiRate, err := currency.GetCurrencyRate(currencyCode, dateStr)
		if err != nil {
			// Skip this date if there's an error (might be weekend/holiday)
			continue
		}

		// Convert API rate to DB format and add to history
		dbRate := storage.CurrencyRate{
			Date:         d,
			CurrencyCode: apiRate.CharCode,
			CurrencyName: apiRate.Name,
			Nominal:      apiRate.Nominal,
			Value:        apiRate.Value,
			Previous:     apiRate.Previous,
		}
		history = append(history, dbRate)

		// Save to database in background
		go func(dateStr string, date time.Time, currentRate *currency.Valute) {
			// Get all rates for this date
			rates, err := currency.GetCBRRatesByDate(dateStr)
			if err == nil {
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

				// Save to database
				if err := db.SaveCurrencyRates(dbRates); err != nil {
					fmt.Printf("Failed to save currency rates to database: %v\n", err)
				}
			} else {
				// If we couldn't get all rates, at least save this one
				dbRate := storage.CurrencyRate{
					Date:         date,
					CurrencyCode: currentRate.CharCode,
					CurrencyName: currentRate.Name,
					Nominal:      currentRate.Nominal,
					Value:        currentRate.Value,
					Previous:     currentRate.Previous,
				}
				if err := db.SaveCurrencyRates([]storage.CurrencyRate{dbRate}); err != nil {
					fmt.Printf("Failed to save currency rate to database: %v\n", err)
				}
			}
		}(dateStr, d, apiRate)
	}

	// If no data found, return error
	if len(history) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "No data found for the specified currency and date range",
		})
		return
	}

	// Create a new Excel file
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println("Error closing Excel file:", err)
		}
	}()

	// Create a new sheet
	sheetName := fmt.Sprintf("%s_rates", currencyCode)
	index, err := f.NewSheet(sheetName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Failed to create Excel sheet",
		})
		return
	}

	// Set headers
	headers := []string{"Date", "Currency Code", "Currency Name", "Nominal", "Value", "Previous Value"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c%d", 'A'+i, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// Set column width
	f.SetColWidth(sheetName, "A", "A", 12)
	f.SetColWidth(sheetName, "B", "B", 15)
	f.SetColWidth(sheetName, "C", "C", 30)
	f.SetColWidth(sheetName, "D", "D", 10)
	f.SetColWidth(sheetName, "E", "E", 12)
	f.SetColWidth(sheetName, "F", "F", 15)

	// Create a style for the header row
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 12,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E0EBF5"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err == nil {
		f.SetCellStyle(sheetName, "A1", "F1", headerStyle)
	}

	// Create a style for data rows
	dataStyle, err := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	// Sort history by date (descending - newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].Date.After(history[j].Date)
	})

	// Fill data
	for i, rate := range history {
		row := i + 2 // Start from row 2 (after header)
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), rate.Date.Format("2006-01-02"))
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), rate.CurrencyCode)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), rate.CurrencyName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), rate.Nominal)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), rate.Value)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), rate.Previous)

		// Apply style to data row
		if err == nil {
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row), dataStyle)
		}
	}

	// Set active sheet
	f.SetActiveSheet(index)

	// Set response headers for file download
	fileName := fmt.Sprintf("%s_rates_%s_to_%s.xlsx", currencyCode, startDateStr, endDateStr)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))

	// Write the Excel file to the response
	if err := f.Write(w); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Failed to generate Excel file",
		})
		return
	}
}

// GetCryptoHistoryHandler handles requests for getting cryptocurrency historical data
func GetCryptoHistoryHandler(w http.ResponseWriter, r *http.Request) {
	// Get symbol parameter from request
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Symbol parameter is required",
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
			Error:   "Days parameter is required",
		})
		return
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 || days > 365 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Days parameter must be a positive integer not exceeding 365",
		})
		return
	}

	// Get database from context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if !ok || db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Database connection not available",
		})
		return
	}

	// Calculate date range
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	// Try to get data from database
	// Use symbol with /RUB suffix as this format is used in Binance API
	dbSymbol := symbol + "/RUB"
	rates, err := db.GetCryptoRatesByDateRange(dbSymbol, startTime, endTime)

	// If data is not found in DB or there was an error, request from Binance API
	if err != nil || len(rates) == 0 {
		// If no data in database, fetch from Binance API
		binanceClient := binance.NewClient()

		// Determine appropriate interval based on days
		var interval binance.KlineInterval
		switch {
		case days <= 1:
			interval = binance.Interval1m
		case days <= 7:
			interval = binance.Interval15m
		case days <= 30:
			interval = binance.Interval1h
		case days <= 90:
			interval = binance.Interval4h
		default:
			interval = binance.Interval1d
		}

		// Get historical data from Binance
		cryptoRates, err := binanceClient.GetHistoricalCryptoToRubRates(symbol, interval, startTime, endTime)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Failed to get cryptocurrency rates: " + err.Error(),
			})
			return
		}

		// Convert to storage.CryptoRate format
		dbRates := make([]storage.CryptoRate, len(cryptoRates))
		for i, rate := range cryptoRates {
			dbRates[i] = storage.CryptoRate{
				Timestamp: rate.Timestamp,
				Symbol:    rate.Symbol, 
				Open:      rate.Open,
				High:      rate.High,
				Low:       rate.Low,
				Close:     rate.Close,
				Volume:    rate.Volume,
			}
		}

		// Save to database
		if len(dbRates) > 0 {
			err = db.SaveCryptoRates(dbRates)
			if err != nil {
				// Log the error but continue
				fmt.Printf("Failed to save crypto rates to database: %v\n", err)
			}
		}

		// Format response
		type HistoricalCryptoRate struct {
			Timestamp int64   `json:"timestamp"`
			Date      string  `json:"date"`
			Open      float64 `json:"open"`
			High      float64 `json:"high"`
			Low       float64 `json:"low"`
			Close     float64 `json:"close"`
			Volume    float64 `json:"volume"`
		}

		result := make([]HistoricalCryptoRate, len(cryptoRates))
		for i, rate := range cryptoRates {
			result[i] = HistoricalCryptoRate{
				Timestamp: rate.Timestamp.Unix() * 1000, // Convert to milliseconds for JavaScript
				Date:      rate.Timestamp.Format("2006-01-02 15:04:05"),
				Open:      rate.Open,
				High:      rate.High,
				Low:       rate.Low,
				Close:     rate.Close,
				Volume:    rate.Volume,
			}
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{
			Success: true,
			Data:    result,
		})
		return
	}

	// Format database response
	type HistoricalCryptoRate struct {
		Timestamp int64   `json:"timestamp"`
		Date      string  `json:"date"`
		Open      float64 `json:"open"`
		High      float64 `json:"high"`
		Low       float64 `json:"low"`
		Close     float64 `json:"close"`
		Volume    float64 `json:"volume"`
	}

	result := make([]HistoricalCryptoRate, len(rates))
	for i, rate := range rates {
		result[i] = HistoricalCryptoRate{
			Timestamp: rate.Timestamp.Unix() * 1000, // Convert to milliseconds for JavaScript
			Date:      rate.Timestamp.Format("2006-01-02 15:04:05"),
			Open:      rate.Open,
			High:      rate.High,
			Low:       rate.Low,
			Close:     rate.Close,
			Volume:    rate.Volume,
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetCryptoHistoryByDateRangeHandler handles requests for getting cryptocurrency historical data by date range
func GetCryptoHistoryByDateRangeHandler(w http.ResponseWriter, r *http.Request) {
	// Get symbol parameter from request
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Symbol parameter is required",
		})
		return
	}

	// Get start_date parameter from request
	startDateStr := r.URL.Query().Get("start_date")
	if startDateStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Start date parameter is required",
		})
		return
	}

	// Get end_date parameter from request
	endDateStr := r.URL.Query().Get("end_date")
	if endDateStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "End date parameter is required",
		})
		return
	}

	// Parse dates
	startTime, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid start date format. Use YYYY-MM-DD",
		})
		return
	}

	endTime, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid end date format. Use YYYY-MM-DD",
		})
		return
	}

	// Add one day to end date to include the entire day
	endTime = endTime.AddDate(0, 0, 1)

	// Check if date range is valid
	if endTime.Before(startTime) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "End date must be after start date",
		})
		return
	}

	// Check if date range is within limits (max 365 days)
	if endTime.Sub(startTime) > 365*24*time.Hour {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Date range cannot exceed 365 days",
		})
		return
	}

	// Get database from context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if !ok || db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Database connection not available",
		})
		return
	}

	// Try to get data from database
	// Use symbol with /RUB suffix as this format is used in Binance API
	dbSymbol := symbol + "/RUB"
	rates, err := db.GetCryptoRatesByDateRange(dbSymbol, startTime, endTime)

	// If data is not found in DB or there was an error, request from Binance API
	if err != nil || len(rates) == 0 {
		// If no data in database, fetch from Binance API
		binanceClient := binance.NewClient()

		// Determine appropriate interval based on date range
		days := int(endTime.Sub(startTime).Hours() / 24)
		var interval binance.KlineInterval
		switch {
		case days <= 1:
			interval = binance.Interval1m
		case days <= 7:
			interval = binance.Interval15m
		case days <= 30:
			interval = binance.Interval1h
		case days <= 90:
			interval = binance.Interval4h
		default:
			interval = binance.Interval1d
		}

		// Get historical data from Binance
		cryptoRates, err := binanceClient.GetHistoricalCryptoToRubRates(symbol, interval, startTime, endTime)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Failed to get cryptocurrency rates: " + err.Error(),
			})
			return
		}

		// Log data received from Binance API
		fmt.Printf("Received %d rates from Binance API for %s from %s to %s\n",
			len(cryptoRates),
			symbol,
			startTime.Format("2006-01-02"),
			endTime.Format("2006-01-02"))

		if len(cryptoRates) > 0 {
			fmt.Printf("First rate: Symbol=%s, Timestamp=%s\n",
				cryptoRates[0].Symbol,
				cryptoRates[0].Timestamp.Format("2006-01-02 15:04:05"))
		}

		// Convert to storage.CryptoRate format
		dbRates := make([]storage.CryptoRate, len(cryptoRates))
		for i, rate := range cryptoRates {
			dbRates[i] = storage.CryptoRate{
				Timestamp: rate.Timestamp,
				Symbol:    rate.Symbol, // Symbol already contains the /RUB suffix
				Open:      rate.Open,
				High:      rate.High,
				Low:       rate.Low,
				Close:     rate.Close,
				Volume:    rate.Volume,
			}
		}

		// Save to database
		if len(dbRates) > 0 {
			err = db.SaveCryptoRates(dbRates)
			if err != nil {
				// Log the error but continue
				fmt.Printf("Failed to save crypto rates to database: %v\n", err)
			} else {
				fmt.Printf("Successfully saved %d rates to database\n", len(dbRates))
			}
		}

		// Format response
		type HistoricalCryptoRate struct {
			Timestamp int64   `json:"timestamp"`
			Date      string  `json:"date"`
			Open      float64 `json:"open"`
			High      float64 `json:"high"`
			Low       float64 `json:"low"`
			Close     float64 `json:"close"`
			Volume    float64 `json:"volume"`
		}

		result := make([]HistoricalCryptoRate, len(cryptoRates))
		for i, rate := range cryptoRates {
			result[i] = HistoricalCryptoRate{
				Timestamp: rate.Timestamp.Unix() * 1000, // Convert to milliseconds for JavaScript
				Date:      rate.Timestamp.Format("2006-01-02 15:04:05"),
				Open:      rate.Open,
				High:      rate.High,
				Low:       rate.Low,
				Close:     rate.Close,
				Volume:    rate.Volume,
			}
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{
			Success: true,
			Data:    result,
		})
		return
	}

	// Format database response
	type HistoricalCryptoRate struct {
		Timestamp int64   `json:"timestamp"`
		Date      string  `json:"date"`
		Open      float64 `json:"open"`
		High      float64 `json:"high"`
		Low       float64 `json:"low"`
		Close     float64 `json:"close"`
		Volume    float64 `json:"volume"`
	}

	result := make([]HistoricalCryptoRate, len(rates))
	for i, rate := range rates {
		result[i] = HistoricalCryptoRate{
			Timestamp: rate.Timestamp.Unix() * 1000, // Convert to milliseconds for JavaScript
			Date:      rate.Timestamp.Format("2006-01-02 15:04:05"),
			Open:      rate.Open,
			High:      rate.High,
			Low:       rate.Low,
			Close:     rate.Close,
			Volume:    rate.Volume,
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetAvailableCryptoSymbolsHandler handles requests for getting available cryptocurrency symbols
func GetAvailableCryptoSymbolsHandler(w http.ResponseWriter, r *http.Request) {
	// Get database from context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if !ok || db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Database connection not available",
		})
		return
	}

	// Get available symbols from database
	symbols, err := db.GetAvailableCryptoSymbols()
	if err != nil || len(symbols) == 0 {
		// If no symbols in database, return default list
		defaultSymbols := []string{"BTC", "ETH", "BNB", "SOL", "XRP", "ADA", "DOGE", "MATIC", "DOT", "LTC"}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{
			Success: true,
			Data:    defaultSymbols,
		})
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    symbols,
	})
}

// ExportCryptoHistoryToExcelHandler handles requests for exporting cryptocurrency history to Excel
func ExportCryptoHistoryToExcelHandler(w http.ResponseWriter, r *http.Request) {
	// Get symbol parameter from request
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Symbol parameter is required",
		})
		return
	}

	// Get start_date parameter from request
	startDateStr := r.URL.Query().Get("start_date")
	if startDateStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Start date parameter is required",
		})
		return
	}

	// Get end_date parameter from request
	endDateStr := r.URL.Query().Get("end_date")
	if endDateStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "End date parameter is required",
		})
		return
	}

	// Parse dates
	startTime, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid start date format. Use YYYY-MM-DD",
		})
		return
	}

	endTime, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid end date format. Use YYYY-MM-DD",
		})
		return
	}

	// Add one day to end date to include the entire day
	endTime = endTime.AddDate(0, 0, 1)

	// Check if date range is valid
	if endTime.Before(startTime) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "End date must be after start date",
		})
		return
	}

	// Check if date range is within limits (max 365 days)
	if endTime.Sub(startTime) > 365*24*time.Hour {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Date range cannot exceed 365 days",
		})
		return
	}

	// Get database from context
	db, ok := r.Context().Value("db").(*storage.PostgresDB)
	if !ok || db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Database connection not available",
		})
		return
	}

	// Try to get data from database
	// Use symbol with /RUB suffix as this format is used in Binance API
	dbSymbol := symbol + "/RUB"
	rates, err := db.GetCryptoRatesByDateRange(dbSymbol, startTime, endTime)

	// If data is not found in DB or there was an error, request from Binance API
	if err != nil || len(rates) == 0 {
		// If no data in database, fetch from Binance API
		binanceClient := binance.NewClient()

		// Determine appropriate interval based on date range
		days := int(endTime.Sub(startTime).Hours() / 24)
		var interval binance.KlineInterval
		switch {
		case days <= 1:
			interval = binance.Interval1m
		case days <= 7:
			interval = binance.Interval15m
		case days <= 30:
			interval = binance.Interval1h
		case days <= 90:
			interval = binance.Interval4h
		default:
			interval = binance.Interval1d
		}

		// Get historical data from Binance
		cryptoRates, err := binanceClient.GetHistoricalCryptoToRubRates(symbol, interval, startTime, endTime)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Failed to get cryptocurrency rates: " + err.Error(),
			})
			return
		}

		// Convert to storage.CryptoRate format for database
		dbRates := make([]storage.CryptoRate, len(cryptoRates))
		for i, rate := range cryptoRates {
			dbRates[i] = storage.CryptoRate{
				Timestamp: rate.Timestamp,
				Symbol:    rate.Symbol, // Symbol already contains the /RUB suffix
				Open:      rate.Open,
				High:      rate.High,
				Low:       rate.Low,
				Close:     rate.Close,
				Volume:    rate.Volume,
			}
		}

		// Save to database
		if len(dbRates) > 0 {
			err = db.SaveCryptoRates(dbRates)
			if err != nil {
				// Log the error but continue
				fmt.Printf("Failed to save crypto rates to database: %v\n", err)
			}
		}

		// Create Excel file
		file := excelize.NewFile()

		// Create a new sheet
		sheetName := fmt.Sprintf("%s_RUB", symbol)
		index, err := file.NewSheet(sheetName)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Failed to create Excel sheet: " + err.Error(),
			})
			return
		}

		// Set headers
		file.SetCellValue(sheetName, "A1", "Date")
		file.SetCellValue(sheetName, "B1", "Open")
		file.SetCellValue(sheetName, "C1", "High")
		file.SetCellValue(sheetName, "D1", "Low")
		file.SetCellValue(sheetName, "E1", "Close")
		file.SetCellValue(sheetName, "F1", "Volume")

		// Add data
		for i, rate := range cryptoRates {
			row := i + 2 // Start from row 2 (after headers)
			file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), rate.Timestamp.Format("2006-01-02 15:04:05"))
			file.SetCellValue(sheetName, fmt.Sprintf("B%d", row), rate.Open)
			file.SetCellValue(sheetName, fmt.Sprintf("C%d", row), rate.High)
			file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), rate.Low)
			file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), rate.Close)
			file.SetCellValue(sheetName, fmt.Sprintf("F%d", row), rate.Volume)
		}

		// Set active sheet
		file.SetActiveSheet(index)

		// Set headers for file download
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_RUB_%s_%s.xlsx",
			symbol, startDateStr, endDateStr))

		// Write file to response
		if err := file.Write(w); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Failed to write Excel file: " + err.Error(),
			})
			return
		}

		return
	}

	// Create Excel file from database data
	file := excelize.NewFile()

	// Create a new sheet
	sheetName := fmt.Sprintf("%s_RUB", symbol)
	index, err := file.NewSheet(sheetName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Failed to create Excel sheet: " + err.Error(),
		})
		return
	}

	// Set headers
	file.SetCellValue(sheetName, "A1", "Date")
	file.SetCellValue(sheetName, "B1", "Open")
	file.SetCellValue(sheetName, "C1", "High")
	file.SetCellValue(sheetName, "D1", "Low")
	file.SetCellValue(sheetName, "E1", "Close")
	file.SetCellValue(sheetName, "F1", "Volume")

	// Add data
	for i, rate := range rates {
		row := i + 2 // Start from row 2 (after headers)
		file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), rate.Timestamp.Format("2006-01-02 15:04:05"))
		file.SetCellValue(sheetName, fmt.Sprintf("B%d", row), rate.Open)
		file.SetCellValue(sheetName, fmt.Sprintf("C%d", row), rate.High)
		file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), rate.Low)
		file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), rate.Close)
		file.SetCellValue(sheetName, fmt.Sprintf("F%d", row), rate.Volume)
	}

	// Set active sheet
	file.SetActiveSheet(index)

	// Set headers for file download
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_RUB_%s_%s.xlsx",
		symbol, startDateStr, endDateStr))

	// Write file to response
	if err := file.Write(w); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Failed to write Excel file: " + err.Error(),
		})
		return
	}
}
