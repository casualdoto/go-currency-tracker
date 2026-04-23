package load

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// newBenchClient returns an HTTP client with a fresh transport per benchmark,
// avoiding stale connection issues between benchmark functions.
func newBenchClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 16,
			MaxConnsPerHost:     16,
		},
		Timeout: 5 * time.Second,
	}
}

// ─── shared data types ────────────────────────────────────────────────────────

type currencyRate struct {
	CurrencyCode string  `json:"currency_code"`
	CurrencyName string  `json:"currency_name"`
	Nominal      int     `json:"nominal"`
	Value        float64 `json:"value"`
	Previous     float64 `json:"previous"`
}

type cryptoRate struct {
	Symbol   string  `json:"symbol"`
	Close    float64 `json:"close"`
	PriceRUB float64 `json:"price_rub"`
}

// writeJSON serialises v as JSON to w.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ─── benchmark data fixtures ──────────────────────────────────────────────────

// cbr34 returns 34 CBR currency rates (typical daily response size).
func cbr34() []currencyRate {
	codes := []string{
		"AUD", "AZN", "GBP", "AMD", "BYR", "BGN", "BRL", "HUF", "VND",
		"HKD", "GEL", "DKK", "AED", "USD", "EUR", "EGP", "INR", "IDR",
		"KZT", "CAD", "QAR", "KGS", "CNY", "MDL", "NZD", "NOK", "PLN",
		"RON", "XDR", "SGD", "TJS", "THB", "TRY", "TMT",
	}
	rates := make([]currencyRate, len(codes))
	for i, c := range codes {
		rates[i] = currencyRate{
			CurrencyCode: c,
			CurrencyName: "Test Currency " + c,
			Nominal:      1,
			Value:        float64(70 + i),
			Previous:     float64(69 + i),
		}
	}
	return rates
}

// crypto20 returns 20 crypto rates.
func crypto20() []cryptoRate {
	symbols := []string{
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
		"ADAUSDT", "AVAXUSDT", "DOTUSDT", "DOGEUSDT", "LINKUSDT",
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
		"ADAUSDT", "AVAXUSDT", "DOTUSDT", "DOGEUSDT", "LINKUSDT",
	}
	rates := make([]cryptoRate, len(symbols))
	for i, s := range symbols {
		rates[i] = cryptoRate{
			Symbol:   s,
			Close:    float64(100*(i+1)) + 0.5,
			PriceRUB: float64(100*(i+1)) * 90.5,
		}
	}
	return rates
}

// ─── normalisation benchmark (inline logic) ───────────────────────────────────

// normalisedCBRRate mirrors the output of the normalization-service.
type normalisedCBRRate struct {
	Date         time.Time `json:"date"`
	CurrencyCode string    `json:"currency_code"`
	CurrencyName string    `json:"currency_name"`
	Nominal      int       `json:"nominal"`
	ValueRUB     float64   `json:"value_rub"`
	PreviousRUB  float64   `json:"previous_rub"`
}

// normaliseCBR converts a raw CBR slice to normalised form.
// This replicates the core logic of normalization-service for benchmarking.
func normaliseCBR(raw []currencyRate, date time.Time) []normalisedCBRRate {
	out := make([]normalisedCBRRate, len(raw))
	for i, r := range raw {
		out[i] = normalisedCBRRate{
			Date:         date,
			CurrencyCode: r.CurrencyCode,
			CurrencyName: r.CurrencyName,
			Nominal:      r.Nominal,
			ValueRUB:     r.Value,
			PreviousRUB:  r.Previous,
		}
	}
	return out
}

// normalisedCryptoRate mirrors the normalization-service crypto output.
type normalisedCryptoRate struct {
	Symbol   string  `json:"symbol"`
	Close    float64 `json:"close"`
	PriceRUB float64 `json:"price_rub"`
}

// normaliseCrypto converts raw crypto rates using a USD/RUB rate.
func normaliseCrypto(raw []cryptoRate, usdRUB float64) []normalisedCryptoRate {
	out := make([]normalisedCryptoRate, len(raw))
	for i, r := range raw {
		out[i] = normalisedCryptoRate{
			Symbol:   r.Symbol,
			Close:    r.Close,
			PriceRUB: r.Close * usdRUB,
		}
	}
	return out
}

// ─── Normalisation benchmarks ──────────────────────────────────────────────────

// BenchmarkNormalization_CBRBuildNormalized measures the throughput of
// converting 34 raw CBR rates to normalised form (JSON encode included).
func BenchmarkNormalization_CBRBuildNormalized(b *testing.B) {
	raw := cbr34()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	data, _ := json.Marshal(raw)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := normaliseCBR(raw, date)
		if _, err := json.Marshal(result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNormalization_CryptoBuildNormalized measures crypto normalisation
// throughput including USD/RUB rate fetch from a mock HTTP server.
func BenchmarkNormalization_CryptoBuildNormalized(b *testing.B) {
	// Stub USD/RUB rate server
	usdRUBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"Valute": map[string]any{
				"USD": map[string]any{"Value": 90.5},
			},
		})
	}))
	defer usdRUBServer.Close()

	raw := crypto20()
	data, _ := json.Marshal(raw)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	client := newBenchClient()
	for i := 0; i < b.N; i++ {
		// Fetch USD/RUB (simulating the normalization-service behaviour)
		resp, err := client.Get(usdRUBServer.URL)
		if err != nil {
			b.Fatal(err)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		usdRUB := 90.5
		normalised := normaliseCrypto(raw, usdRUB)
		if _, err := json.Marshal(normalised); err != nil {
			b.Fatal(err)
		}
	}
}

// ─── History-service benchmarks ───────────────────────────────────────────────

// newHistoryServer returns a minimal httptest.Server simulating history-service endpoints.
func newHistoryServer() *httptest.Server {
	cbrData, _ := json.Marshal(cbr34())
	cryptoData, _ := json.Marshal(crypto20())
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
		"ADAUSDT", "AVAXUSDT", "DOTUSDT", "DOGEUSDT", "LINKUSDT"}
	symbolsData, _ := json.Marshal(symbols)

	mux := http.NewServeMux()
	mux.HandleFunc("/history/cbr", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cbrData)
	})
	mux.HandleFunc("/history/crypto", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cryptoData)
	})
	mux.HandleFunc("/history/crypto/symbols", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(symbolsData)
	})
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return httptest.NewServer(mux)
}

// BenchmarkHistoryService_GetCBRHistory measures sequential GET /history/cbr throughput.
func BenchmarkHistoryService_GetCBRHistory(b *testing.B) {
	srv := newHistoryServer()
	defer srv.Close()
	url := srv.URL + "/history/cbr?date=2024-01-15"
	client := newBenchClient()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkHistoryService_GetCBRHistory_Parallel measures concurrent GET /history/cbr throughput.
func BenchmarkHistoryService_GetCBRHistory_Parallel(b *testing.B) {
	srv := newHistoryServer()
	defer srv.Close()
	url := srv.URL + "/history/cbr?date=2024-01-15"

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     32,
			MaxIdleConnsPerHost: 32,
		},
		Timeout: 5 * time.Second,
	}
	b.SetParallelism(8)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(url)
			if err != nil {
				b.Error(err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

// BenchmarkHistoryService_GetCryptoSymbols measures the lightweight symbols endpoint.
func BenchmarkHistoryService_GetCryptoSymbols(b *testing.B) {
	srv := newHistoryServer()
	defer srv.Close()
	url := srv.URL + "/history/crypto/symbols"
	client := newBenchClient()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkHistoryService_GetCryptoHistory measures GET /history/crypto with 20 records.
func BenchmarkHistoryService_GetCryptoHistory(b *testing.B) {
	srv := newHistoryServer()
	defer srv.Close()
	url := srv.URL + "/history/crypto?symbol=BTCUSDT"
	client := newBenchClient()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// ─── API Gateway benchmarks ───────────────────────────────────────────────────

// newGatewayStack returns a gateway httptest.Server pointing at a history stub.
func newGatewayStack() (gw *httptest.Server, history *httptest.Server) {
	history = newHistoryServer()

	client := &http.Client{Timeout: 10 * time.Second}

	forward := func(w http.ResponseWriter, r *http.Request, targetURL string) {
		req, _ := http.NewRequest(r.Method, targetURL, nil)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("pong"))
	})
	mux.HandleFunc("/history/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.RequestURI(), "/history")
		forward(w, r, history.URL+"/history"+path)
	})
	mux.HandleFunc("/rates/cbr", func(w http.ResponseWriter, r *http.Request) {
		forward(w, r, history.URL+"/history/cbr?"+r.URL.RawQuery)
	})

	gw = httptest.NewServer(mux)
	return
}

// BenchmarkAPIGateway_Throughput measures end-to-end proxy throughput:
// benchmark client → gateway → history stub.
func BenchmarkAPIGateway_Throughput(b *testing.B) {
	gw, history := newGatewayStack()
	defer gw.Close()
	defer history.Close()
	url := gw.URL + "/history/cbr?date=2024-01-15"
	client := newBenchClient()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkAPIGateway_ConcurrentRequests measures the gateway under concurrent load.
// Uses a shared client with bounded connection pool to avoid thread exhaustion.
func BenchmarkAPIGateway_ConcurrentRequests(b *testing.B) {
	gw, history := newGatewayStack()
	defer gw.Close()
	defer history.Close()
	url := gw.URL + "/ping"

	// Shared client with bounded pool — critical for concurrent benchmarks.
	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     32,
			MaxIdleConnsPerHost: 32,
		},
		Timeout: 5 * time.Second,
	}

	b.SetParallelism(8) // 8 goroutines per CPU core
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(url)
			if err != nil {
				b.Error(err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

// ─── Notification-service benchmarks ─────────────────────────────────────────

// inMemNotifStore is a minimal thread-safe subscription store for benchmarks.
type inMemNotifStore struct {
	mu   sync.RWMutex
	cbr  map[int64]map[string]struct{}
}

func newInMemNotifStore() *inMemNotifStore {
	return &inMemNotifStore{cbr: make(map[int64]map[string]struct{})}
}

func (s *inMemNotifStore) subscribe(id int64, cur string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cbr[id] == nil {
		s.cbr[id] = make(map[string]struct{})
	}
	s.cbr[id][cur] = struct{}{}
}

func (s *inMemNotifStore) unsubscribe(id int64, cur string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cbr[id], cur)
}

func (s *inMemNotifStore) list(id int64) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for k := range s.cbr[id] {
		out = append(out, k)
	}
	return out
}

// newNotifServer creates an httptest.Server for the notification service benchmark.
func newNotifServer(store *inMemNotifStore) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/subscriptions/cbr", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				TelegramID int64  `json:"telegram_id"`
				Value      string `json:"value"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			store.subscribe(body.TelegramID, body.Value)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			var body struct {
				TelegramID int64  `json:"telegram_id"`
				Value      string `json:"value"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			store.unsubscribe(body.TelegramID, body.Value)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			tidStr := r.URL.Query().Get("telegram_id")
			tid, _ := strconv.ParseInt(tidStr, 10, 64)
			writeJSON(w, http.StatusOK, store.list(tid))
		}
	})
	return httptest.NewServer(mux)
}

// BenchmarkNotification_SubscribeUnsubscribe measures the full subscribe→unsubscribe cycle.
func BenchmarkNotification_SubscribeUnsubscribe(b *testing.B) {
	store := newInMemNotifStore()
	srv := newNotifServer(store)
	defer srv.Close()

	client := newBenchClient()
	url := srv.URL + "/subscriptions/cbr"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		userID := int64(i % 1000)

		// Subscribe
		subBody, _ := json.Marshal(map[string]any{
			"telegram_id": userID, "value": "USD",
		})
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(subBody))
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()

		// Unsubscribe
		delBody, _ := json.Marshal(map[string]any{
			"telegram_id": userID, "value": "USD",
		})
		req, _ = http.NewRequest(http.MethodDelete, url, bytes.NewReader(delBody))
		resp, err = client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkNotification_ListSubscriptions measures listing subscriptions for a user
// with 10 pre-seeded entries.
func BenchmarkNotification_ListSubscriptions(b *testing.B) {
	store := newInMemNotifStore()
	currencies := []string{"USD", "EUR", "GBP", "JPY", "CHF", "CAD", "AUD", "CNY", "HKD", "SGD"}
	for _, c := range currencies {
		store.subscribe(42, c)
	}
	srv := newNotifServer(store)
	defer srv.Close()

	url := fmt.Sprintf("%s/subscriptions/cbr?telegram_id=42", srv.URL)
	client := newBenchClient()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
