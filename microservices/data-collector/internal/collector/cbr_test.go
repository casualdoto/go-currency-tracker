package collector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/data-collector/internal/producer"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// stubCBRServer returns an httptest.Server that responds with a serialised cbrResponse.
func stubCBRServer(t *testing.T, resp cbrResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
}

func sampleValute(charCode string, value, previous float64) cbrValute {
	return cbrValute{
		CharCode: charCode,
		NumCode:  "840",
		Nominal:  1,
		Name:     charCode + " test",
		Value:    value,
		Previous: previous,
	}
}

// ─── parseCBRResponse ─────────────────────────────────────────────────────────

func TestParseCBRResponse_basic(t *testing.T) {
	data := cbrResponse{
		Date: "2026/04/15 11:30:00",
		Valute: map[string]cbrValute{
			"USD": sampleValute("USD", 90.5, 89.0),
			"EUR": sampleValute("EUR", 98.2, 97.0),
		},
	}
	now := time.Now()
	rates := parseCBRResponse(data, now)

	if len(rates) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(rates))
	}

	byCode := make(map[string]rawCBRRate, len(rates))
	for _, r := range rates {
		byCode[r.CharCode] = r
	}

	usd, ok := byCode["USD"]
	if !ok {
		t.Fatal("USD not found in result")
	}
	if usd.Value != 90.5 {
		t.Errorf("USD Value: expected 90.5, got %f", usd.Value)
	}
	if usd.Previous != 89.0 {
		t.Errorf("USD Previous: expected 89.0, got %f", usd.Previous)
	}
	if usd.Nominal != 1 {
		t.Errorf("USD Nominal: expected 1, got %d", usd.Nominal)
	}
	if usd.CollectedAt.IsZero() {
		t.Error("CollectedAt should not be zero")
	}
}

func TestParseCBRResponse_emptyValute(t *testing.T) {
	data := cbrResponse{
		Date:   "2026/04/15 11:30:00",
		Valute: map[string]cbrValute{},
	}
	rates := parseCBRResponse(data, time.Now())

	if rates == nil {
		t.Error("expected non-nil slice for empty Valute")
	}
	if len(rates) != 0 {
		t.Errorf("expected 0 rates, got %d", len(rates))
	}
}

func TestParseCBRResponse_preservesDate(t *testing.T) {
	wantDate := "2026/04/15 11:30:00"
	data := cbrResponse{
		Date:   wantDate,
		Valute: map[string]cbrValute{"USD": sampleValute("USD", 90, 89)},
	}
	rates := parseCBRResponse(data, time.Now())

	if len(rates) != 1 {
		t.Fatalf("expected 1 rate, got %d", len(rates))
	}
	if rates[0].Date != wantDate {
		t.Errorf("Date: expected %q, got %q", wantDate, rates[0].Date)
	}
}

func TestParseCBRResponse_multipleRates(t *testing.T) {
	valutes := map[string]cbrValute{
		"USD": sampleValute("USD", 90, 89),
		"EUR": sampleValute("EUR", 98, 97),
		"GBP": sampleValute("GBP", 114, 113),
		"CNY": sampleValute("CNY", 12, 11),
		"JPY": sampleValute("JPY", 0.6, 0.59),
	}
	data := cbrResponse{Date: "2026/04/15 11:30:00", Valute: valutes}
	rates := parseCBRResponse(data, time.Now())

	if len(rates) != 5 {
		t.Errorf("expected 5 rates, got %d", len(rates))
	}
}

// ─── CBRCollector.Collect ─────────────────────────────────────────────────────

func TestCBRCollector_Collect_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close() // закрываем до запроса

	prod := producer.New("localhost:1") // недоступный брокер
	c := NewCBR(srvURL, prod)
	err := c.Collect()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cbr fetch") {
		t.Errorf("expected error to contain 'cbr fetch', got: %v", err)
	}
}

func TestCBRCollector_Collect_non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	prod := producer.New("localhost:1")
	c := NewCBR(srv.URL, prod)
	err := c.Collect()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cbr status 500") {
		t.Errorf("expected error to contain 'cbr status 500', got: %v", err)
	}
}

func TestCBRCollector_Collect_invalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json at all"))
	}))
	defer srv.Close()

	prod := producer.New("localhost:1")
	c := NewCBR(srv.URL, prod)
	err := c.Collect()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cbr decode") {
		t.Errorf("expected error to contain 'cbr decode', got: %v", err)
	}
}

func TestCBRCollector_Collect_parsesAndReachesPublish(t *testing.T) {
	// Upstream возвращает корректный JSON с двумя валютами.
	// Producer указывает на несуществующий брокер — публикация упадёт,
	// но парсинг уже прошёл успешно.
	srv := stubCBRServer(t, cbrResponse{
		Date: "2026/04/15 11:30:00",
		Valute: map[string]cbrValute{
			"USD": sampleValute("USD", 90.5, 89.0),
			"EUR": sampleValute("EUR", 98.2, 97.0),
		},
	})
	defer srv.Close()

	prod := producer.New("localhost:1")
	c := NewCBR(srv.URL, prod)
	err := c.Collect()

	if err == nil {
		t.Fatal("expected error (kafka unavailable), got nil")
	}
	if !strings.Contains(err.Error(), "cbr publish") {
		t.Errorf("expected error to contain 'cbr publish' (parse succeeded, kafka failed), got: %v", err)
	}
}
