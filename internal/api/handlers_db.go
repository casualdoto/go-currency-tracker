package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// GetStoredRatesHandler returns currency rates from the database
func (h *DatabaseHandler) GetStoredRatesHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	dateStr := r.URL.Query().Get("date")

	var date time.Time
	var err error

	// If date is not provided, use current date
	if dateStr == "" {
		date = time.Now().Truncate(24 * time.Hour)
	} else {
		// Parse date from string
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "Invalid date format. Use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	// Get rates from database
	rates, err := h.DB.GetCurrencyRatesByDate(date)
	if err != nil {
		http.Error(w, "Failed to get currency rates from database", http.StatusInternalServerError)
		return
	}

	// If no rates found for the date
	if len(rates) == 0 {
		http.Error(w, "No currency rates found for the specified date", http.StatusNotFound)
		return
	}

	// Convert to response format
	response := make(map[string]interface{})
	response["date"] = date.Format("2006-01-02")
	response["timestamp"] = time.Now().Unix()

	// Create valute map
	valute := make(map[string]map[string]interface{})
	for _, rate := range rates {
		valute[rate.CurrencyCode] = map[string]interface{}{
			"ID":       rate.CurrencyCode,
			"NumCode":  "",
			"CharCode": rate.CurrencyCode,
			"Nominal":  rate.Nominal,
			"Name":     rate.CurrencyName,
			"Value":    rate.Value,
			"Previous": rate.Previous,
		}
	}
	response["Valute"] = valute

	// Set content type and encode response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetStoredCurrencyRateHandler returns a specific currency rate from the database
func (h *DatabaseHandler) GetStoredCurrencyRateHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Currency code is required", http.StatusBadRequest)
		return
	}

	dateStr := r.URL.Query().Get("date")
	var date time.Time
	var err error

	// If date is not provided, use current date
	if dateStr == "" {
		date = time.Now().Truncate(24 * time.Hour)
	} else {
		// Parse date from string
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "Invalid date format. Use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	// Get rate from database
	rate, err := h.DB.GetCurrencyRate(code, date)
	if err != nil {
		http.Error(w, "Currency rate not found", http.StatusNotFound)
		return
	}

	// Create response
	response := map[string]interface{}{
		"date":      date.Format("2006-01-02"),
		"timestamp": time.Now().Unix(),
		"currency": map[string]interface{}{
			"code":     rate.CurrencyCode,
			"name":     rate.CurrencyName,
			"nominal":  rate.Nominal,
			"value":    rate.Value,
			"previous": rate.Previous,
		},
	}

	// Set content type and encode response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetAvailableDatesHandler returns a list of dates for which currency rates are available
func (h *DatabaseHandler) GetAvailableDatesHandler(w http.ResponseWriter, r *http.Request) {
	// Get available dates from database
	dates, err := h.DB.GetAvailableDates()
	if err != nil {
		http.Error(w, "Failed to get available dates", http.StatusInternalServerError)
		return
	}

	// If no dates found
	if len(dates) == 0 {
		// Return at least current date
		dates = []time.Time{time.Now().Truncate(24 * time.Hour)}
	}

	// Format dates to strings
	var dateStrings []string
	for _, date := range dates {
		dateStrings = append(dateStrings, date.Format("2006-01-02"))
	}

	// Create response
	response := map[string]interface{}{
		"dates": dateStrings,
	}

	// Set content type and encode response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
