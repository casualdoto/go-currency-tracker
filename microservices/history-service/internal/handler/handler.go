package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/cbrbackfill"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/cryptobackfill"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

type Handler struct {
	pg     *storage.PostgresDB
	ch     *storage.ClickHouseDB
	cbr    *cbrbackfill.Client
	crypto *cryptobackfill.Client
}

func New(pg *storage.PostgresDB, ch *storage.ClickHouseDB, cbr *cbrbackfill.Client, crypto *cryptobackfill.Client) *Handler {
	return &Handler{pg: pg, ch: ch, cbr: cbr, crypto: crypto}
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

	rates, err := h.pg.GetCurrencyRatesByDate(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if len(rates) == 0 {
		h.backfillCBRDayIfEmpty(date)
		rates, err = h.pg.GetCurrencyRatesByDate(date)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
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

	rates, err := h.pg.GetCurrencyRatesByDateRange(code, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if h.backfillCBRMissingDays(code, from, to) {
		rates, err = h.pg.GetCurrencyRatesByDateRange(code, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
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
	rates, err := h.ch.GetCryptoRatesBySymbol(symbol, 100)
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

	// Short ranges: same resolution as monolith (15m for ≤7 days, etc.) from Binance + USDTRUB/CBR.
	// Avoids mixing 1d backfill + live USDT Close rows (UI would treat Close as RUB and fake "zero" dips).
	span := inclusiveCalendarDaysUTC(from, to)
	interval := cryptobackfill.IntervalForCalendarSpan(span)
	if interval != "1d" && h.crypto != nil {
		live, err := h.crypto.FetchIntervalRUBRates(symbol, interval, from, to)
		if err == nil && len(live) > 0 {
			rows := append([]storage.CryptoRate(nil), live...)
			go func() {
				if err := h.ch.SaveCryptoRates(rows); err != nil {
					log.Printf("crypto: async cache to clickhouse: %v", err)
				}
			}()
			writeJSON(w, http.StatusOK, live)
			return
		}
		if err != nil {
			log.Printf("crypto: binance interval %s failed, using DB: %v", interval, err)
		}
	}

	rates, err := h.ch.GetCryptoRatesByDateRange(symbol, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if h.backfillCryptoRange(symbol, from, to, rates) {
		rates, err = h.ch.GetCryptoRatesByDateRange(symbol, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	writeJSON(w, http.StatusOK, rates)
}

// GET /history/crypto/symbols
func (h *Handler) GetCryptoSymbols(w http.ResponseWriter, r *http.Request) {
	symbols, err := h.ch.GetAvailableCryptoSymbols()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, symbols)
}
