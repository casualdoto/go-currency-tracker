//go:build integration

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
	"github.com/go-chi/chi/v5"
)

// newIntegrationServer creates a real httptest.Server backed by the provided stubs.
// The server mounts the same routes as the production history-service.
func newIntegrationServer(t *testing.T, pg *stubPG, ch *stubCH) *httptest.Server {
	t.Helper()
	h := &testableHandler{pg: pg, ch: ch}
	r := chi.NewRouter()
	r.Get("/history/cbr", h.GetCBRHistory)
	r.Get("/history/crypto", h.GetCryptoHistory)
	r.Get("/history/crypto/symbols", h.GetCryptoSymbols)
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// TestIntegration_GetCBRHistory_fullCycle verifies a complete HTTP round-trip:
// request → chi router → testableHandler → stub DB → JSON response.
func TestIntegration_GetCBRHistory_fullCycle(t *testing.T) {
	pg := &stubPG{rates: []storage.CurrencyRate{
		{CurrencyCode: "USD", Value: 90.5},
		{CurrencyCode: "EUR", Value: 98.2},
	}}
	srv := newIntegrationServer(t, pg, &stubCH{})

	resp, err := http.Get(fmt.Sprintf("%s/history/cbr?date=2024-01-15", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rates []storage.CurrencyRate
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(rates) != 2 {
		t.Errorf("expected 2 rates, got %d", len(rates))
	}
	if rates[0].CurrencyCode != "USD" && rates[1].CurrencyCode != "USD" {
		t.Error("expected USD in rates")
	}
}

// TestIntegration_GetCBRHistory_noDate_defaultsToday verifies that omitting
// the date parameter returns a 200 (uses today's date internally).
func TestIntegration_GetCBRHistory_noDate_defaultsToday(t *testing.T) {
	pg := &stubPG{rates: []storage.CurrencyRate{
		{CurrencyCode: "CNY", Value: 12.5},
	}}
	srv := newIntegrationServer(t, pg, &stubCH{})

	resp, err := http.Get(fmt.Sprintf("%s/history/cbr", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestIntegration_GetCBRHistory_invalidDate_returns400 verifies that an
// invalid date format yields a 400 Bad Request.
func TestIntegration_GetCBRHistory_invalidDate_returns400(t *testing.T) {
	srv := newIntegrationServer(t, &stubPG{}, &stubCH{})

	resp, err := http.Get(fmt.Sprintf("%s/history/cbr?date=not-a-date", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestIntegration_GetCryptoHistory_missingSymbol_returns400 verifies that
// omitting the required symbol parameter yields a 400.
func TestIntegration_GetCryptoHistory_missingSymbol_returns400(t *testing.T) {
	srv := newIntegrationServer(t, &stubPG{}, &stubCH{})

	resp, err := http.Get(fmt.Sprintf("%s/history/crypto", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestIntegration_GetCryptoHistory_withSymbol_returnsRates verifies that
// a valid symbol parameter returns the crypto rates.
func TestIntegration_GetCryptoHistory_withSymbol_returnsRates(t *testing.T) {
	ch := &stubCH{rates: []storage.CryptoRate{
		{Symbol: "BTCUSDT", Close: 41000, PriceRUB: 3690000},
		{Symbol: "BTCUSDT", Close: 42000, PriceRUB: 3780000},
	}}
	srv := newIntegrationServer(t, &stubPG{}, ch)

	resp, err := http.Get(fmt.Sprintf("%s/history/crypto?symbol=BTCUSDT", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rates []storage.CryptoRate
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatal(err)
	}
	if len(rates) != 2 {
		t.Errorf("expected 2 rates, got %d", len(rates))
	}
}

// TestIntegration_GetCryptoSymbols_returnsList verifies that the symbols
// endpoint returns the full list of available crypto symbols.
func TestIntegration_GetCryptoSymbols_returnsList(t *testing.T) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT"}
	srv := newIntegrationServer(t, &stubPG{}, &stubCH{symbols: symbols})

	resp, err := http.Get(fmt.Sprintf("%s/history/crypto/symbols", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result) != len(symbols) {
		t.Errorf("expected %d symbols, got %d", len(symbols), len(result))
	}
}

// TestIntegration_ContentTypeJSON verifies that all endpoints return
// Content-Type: application/json regardless of success or error.
func TestIntegration_ContentTypeJSON(t *testing.T) {
	srv := newIntegrationServer(t, &stubPG{}, &stubCH{})

	endpoints := []string{
		"/history/cbr?date=2024-01-15",
		"/history/cbr?date=bad-date",
		"/history/crypto?symbol=BTCUSDT",
		"/history/crypto",
		"/history/crypto/symbols",
	}

	for _, ep := range endpoints {
		resp, err := http.Get(srv.URL + ep)
		if err != nil {
			t.Errorf("%s: request failed: %v", ep, err)
			continue
		}
		resp.Body.Close()
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("%s: expected Content-Type application/json, got %q", ep, ct)
		}
	}
}

// TestIntegration_DBError_returns500 verifies that a database error
// propagates as HTTP 500.
func TestIntegration_DBError_returns500(t *testing.T) {
	errPG := &stubPG{err: fmt.Errorf("connection refused")}
	errCH := &stubCH{err: fmt.Errorf("clickhouse unavailable")}
	srv := newIntegrationServer(t, errPG, errCH)

	tests := []struct {
		path string
	}{
		{"/history/cbr?date=2024-01-15"},
		{"/history/crypto?symbol=BTCUSDT"},
		{"/history/crypto/symbols"},
	}

	for _, tc := range tests {
		resp, err := http.Get(srv.URL + tc.path)
		if err != nil {
			t.Errorf("%s: request failed: %v", tc.path, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("%s: expected 500, got %d", tc.path, resp.StatusCode)
		}
	}
}

// TestIntegration_HealthCheck verifies that the /ping endpoint returns 200.
func TestIntegration_HealthCheck(t *testing.T) {
	srv := newIntegrationServer(t, &stubPG{}, &stubCH{})

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
