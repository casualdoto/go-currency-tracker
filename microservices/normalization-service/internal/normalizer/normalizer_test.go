package normalizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func newTestNormalizer(cbrURL string) *Normalizer {
	return &Normalizer{
		cbrURL:     cbrURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// stubCBRServer returns an httptest.Server that responds with a CBR JSON
// containing a single USD entry with the given value.
func stubCBRServer(t *testing.T, usdValue float64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"Valute": map[string]any{
				"USD": map[string]any{"Value": usdValue},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

// ─── normalizeCBR ─────────────────────────────────────────────────────────────

func TestNormalizeCBR_basic(t *testing.T) {
	n := newTestNormalizer("")

	rates := []rawCBRRate{
		{
			Date:     "2024-01-15T00:00:00+03:00",
			CharCode: "USD",
			Nominal:  1,
			Name:     "Доллар США",
			Value:    90.5,
			Previous: 89.0,
		},
		{
			Date:     "2024-01-15T00:00:00+03:00",
			CharCode: "EUR",
			Nominal:  1,
			Name:     "Евро",
			Value:    98.2,
			Previous: 97.0,
		},
	}

	raw, _ := json.Marshal(rates)
	result, err := n.buildNormalizedCBR(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(result))
	}

	usd := result[0]
	if usd.CurrencyCode != "USD" {
		t.Errorf("expected USD, got %s", usd.CurrencyCode)
	}
	if usd.ValueRUB != 90.5 {
		t.Errorf("expected ValueRUB=90.5, got %f", usd.ValueRUB)
	}
	if usd.PreviousRUB != 89.0 {
		t.Errorf("expected PreviousRUB=89.0, got %f", usd.PreviousRUB)
	}
}

func TestNormalizeCBR_dateFormats(t *testing.T) {
	n := newTestNormalizer("")

	tests := []struct {
		name     string
		dateStr  string
		wantZero bool
	}{
		{"RFC3339", "2024-03-01T00:00:00+03:00", false},
		{"slash format", "2024/03/01 00:00:00", false},
		{"invalid falls back to today", "not-a-date", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rates := []rawCBRRate{{Date: tc.dateStr, CharCode: "USD", Nominal: 1, Name: "USD", Value: 90}}
			raw, _ := json.Marshal(rates)
			result, err := n.buildNormalizedCBR(raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != 1 {
				t.Fatalf("expected 1 result")
			}
			if result[0].Date.IsZero() {
				t.Error("date should not be zero")
			}
		})
	}
}

// ─── normalizeCrypto ──────────────────────────────────────────────────────────

func TestNormalizeCrypto_priceRUB(t *testing.T) {
	srv := stubCBRServer(t, 90.0)
	defer srv.Close()

	n := newTestNormalizer(srv.URL)

	rates := []rawCryptoRate{
		{Symbol: "BTCUSDT", Timestamp: time.Now(), Open: 40000, High: 42000, Low: 39000, Close: 41000, Volume: 1.5},
		{Symbol: "ETHUSDT", Timestamp: time.Now(), Open: 2000, High: 2100, Low: 1950, Close: 2050, Volume: 10},
	}
	raw, _ := json.Marshal(rates)

	result, err := n.buildNormalizedCrypto(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	btc := result[0]
	expectedRUB := 41000.0 * 90.0
	if btc.PriceRUB != expectedRUB {
		t.Errorf("BTC PriceRUB: expected %f, got %f", expectedRUB, btc.PriceRUB)
	}
	if btc.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", btc.Symbol)
	}
}

func TestNormalizeCrypto_cbrUnavailable_fallback(t *testing.T) {
	// Point to a server that always errors.
	n := newTestNormalizer("http://127.0.0.1:1")

	rates := []rawCryptoRate{
		{Symbol: "BTCUSDT", Timestamp: time.Now(), Close: 50000},
	}
	raw, _ := json.Marshal(rates)

	// Should not fail — fallback usdRUB = 1.0
	result, err := n.buildNormalizedCrypto(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].PriceRUB != 50000.0 {
		t.Errorf("expected fallback PriceRUB=50000, got %f", result[0].PriceRUB)
	}
}

// ─── getUSDRUBRate ────────────────────────────────────────────────────────────

func TestGetUSDRUBRate(t *testing.T) {
	srv := stubCBRServer(t, 87.5)
	defer srv.Close()

	n := newTestNormalizer(srv.URL)
	rate, err := n.getUSDRUBRate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 87.5 {
		t.Errorf("expected 87.5, got %f", rate)
	}
}

func TestGetUSDRUBRate_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := newTestNormalizer(srv.URL)
	_, err := n.getUSDRUBRate()
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}
