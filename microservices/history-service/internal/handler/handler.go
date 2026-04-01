package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

type Handler struct {
	db *storage.PostgresDB
}

func New(db *storage.PostgresDB) *Handler {
	return &Handler{db: db}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// GET /history/cbr?date=2024-01-15
func (h *Handler) GetCBRHistory(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	if dateStr == "" {
		date = time.Now().Truncate(24 * time.Hour)
	} else {
		var err error
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
			return
		}
	}

	rates, err := h.db.GetCurrencyRatesByDate(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, rates)
}

// GET /history/cbr/range?code=USD&from=2024-01-01&to=2024-01-31
func (h *Handler) GetCBRHistoryRange(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from date")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to date")
		return
	}

	rates, err := h.db.GetCurrencyRatesByDateRange(code, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, rates)
}

// GET /history/crypto?symbol=BTCUSDT&limit=100
func (h *Handler) GetCryptoHistory(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}
	limit := 100
	rates, err := h.db.GetCryptoRatesBySymbol(symbol, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, rates)
}

// GET /history/crypto/range?symbol=BTCUSDT&from=2024-01-01&to=2024-01-31
func (h *Handler) GetCryptoHistoryRange(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from date")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to date")
		return
	}

	rates, err := h.db.GetCryptoRatesByDateRange(symbol, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, rates)
}

// GET /history/crypto/symbols
func (h *Handler) GetCryptoSymbols(w http.ResponseWriter, r *http.Request) {
	symbols, err := h.db.GetAvailableCryptoSymbols()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, symbols)
}
