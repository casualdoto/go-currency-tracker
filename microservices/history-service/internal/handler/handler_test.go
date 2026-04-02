package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

// ─── stubs ────────────────────────────────────────────────────────────────────

// stubPG and stubCH satisfy the same method signatures as the real DB types,
// letting us test handler logic without real database connections.

type stubPG struct {
	rates []storage.CurrencyRate
	err   error
}

func (s *stubPG) GetCurrencyRatesByDate(_ time.Time) ([]storage.CurrencyRate, error) {
	return s.rates, s.err
}
func (s *stubPG) GetCurrencyRatesByDateRange(_ string, _, _ time.Time) ([]storage.CurrencyRate, error) {
	return s.rates, s.err
}

type stubCH struct {
	rates   []storage.CryptoRate
	symbols []string
	err     error
}

func (s *stubCH) GetCryptoRatesBySymbol(_ string, _ int) ([]storage.CryptoRate, error) {
	return s.rates, s.err
}
func (s *stubCH) GetCryptoRatesByDateRange(_ string, _, _ time.Time) ([]storage.CryptoRate, error) {
	return s.rates, s.err
}
func (s *stubCH) GetAvailableCryptoSymbols() ([]string, error) {
	return s.symbols, s.err
}

// testableHandler wraps the stub interfaces instead of concrete DB types.
type testableHandler struct {
	pg *stubPG
	ch *stubCH
}

func (h *testableHandler) GetCBRHistory(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	if dateStr != "" {
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
	writeJSON(w, http.StatusOK, rates)
}

func (h *testableHandler) GetCryptoHistory(w http.ResponseWriter, r *http.Request) {
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

func (h *testableHandler) GetCryptoSymbols(w http.ResponseWriter, r *http.Request) {
	symbols, err := h.ch.GetAvailableCryptoSymbols()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, symbols)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func get(t *testing.T, fn http.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	fn(rr, req)
	return rr
}

// ─── CBR handler tests ────────────────────────────────────────────────────────

func TestGetCBRHistory_returnRates(t *testing.T) {
	h := &testableHandler{
		pg: &stubPG{rates: []storage.CurrencyRate{
			{CurrencyCode: "USD", Value: 90.5},
			{CurrencyCode: "EUR", Value: 98.2},
		}},
	}

	rr := get(t, h.GetCBRHistory, "/history/cbr?date=2024-01-15")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var rates []storage.CurrencyRate
	json.NewDecoder(rr.Body).Decode(&rates)
	if len(rates) != 2 {
		t.Errorf("expected 2 rates, got %d", len(rates))
	}
}

func TestGetCBRHistory_invalidDate(t *testing.T) {
	h := &testableHandler{pg: &stubPG{}}
	rr := get(t, h.GetCBRHistory, "/history/cbr?date=not-a-date")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetCBRHistory_emptyResult(t *testing.T) {
	h := &testableHandler{pg: &stubPG{rates: nil}}
	rr := get(t, h.GetCBRHistory, "/history/cbr?date=2024-01-15")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ─── Crypto handler tests ─────────────────────────────────────────────────────

func TestGetCryptoHistory_missingSymbol(t *testing.T) {
	h := &testableHandler{ch: &stubCH{}}
	rr := get(t, h.GetCryptoHistory, "/history/crypto")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetCryptoHistory_returnRates(t *testing.T) {
	h := &testableHandler{
		ch: &stubCH{rates: []storage.CryptoRate{
			{Symbol: "BTCUSDT", Close: 41000, PriceRUB: 3690000},
		}},
	}
	rr := get(t, h.GetCryptoHistory, "/history/crypto?symbol=BTCUSDT")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var rates []storage.CryptoRate
	json.NewDecoder(rr.Body).Decode(&rates)
	if len(rates) != 1 {
		t.Errorf("expected 1 rate, got %d", len(rates))
	}
	if rates[0].Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", rates[0].Symbol)
	}
}

func TestGetCryptoSymbols_returnList(t *testing.T) {
	h := &testableHandler{
		ch: &stubCH{symbols: []string{"BTCUSDT", "ETHUSDT"}},
	}
	rr := get(t, h.GetCryptoSymbols, "/history/crypto/symbols")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var symbols []string
	json.NewDecoder(rr.Body).Decode(&symbols)
	if len(symbols) != 2 {
		t.Errorf("expected 2 symbols, got %d", len(symbols))
	}
}
