// Package api provides HTTP request handlers and API route setup.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency/binance"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
	"github.com/xuri/excelize/v2"
)

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

		// Format response
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
	result := make([]HistoricalCryptoRate, len(rates))
	for i, rate := range rates {
		result[i] = convertCryptoRateToHistorical(rate)
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

		// Format response
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
	result := make([]HistoricalCryptoRate, len(rates))
	for i, rate := range rates {
		result[i] = convertCryptoRateToHistorical(rate)
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
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Println("Error closing Excel file:", err)
			}
		}()

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

	// If we have data from database, create Excel file
	file := excelize.NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Println("Error closing Excel file:", err)
		}
	}()

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
