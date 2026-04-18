//go:build e2e

package e2e

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

// ─── in-memory subscription store (Redis replacement) ─────────────────────────

type memStore struct {
	mu     sync.RWMutex
	cbr    map[int64]map[string]struct{}
	crypto map[int64]map[string]struct{}
}

func newMemStore() *memStore {
	return &memStore{
		cbr:    make(map[int64]map[string]struct{}),
		crypto: make(map[int64]map[string]struct{}),
	}
}

func (m *memStore) subscribeCBR(id int64, cur string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cbr[id] == nil {
		m.cbr[id] = make(map[string]struct{})
	}
	m.cbr[id][cur] = struct{}{}
}

func (m *memStore) unsubscribeCBR(id int64, cur string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cbr[id], cur)
}

func (m *memStore) getCBR(id int64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for k := range m.cbr[id] {
		out = append(out, k)
	}
	return out
}

func (m *memStore) subscribeCrypto(id int64, sym string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.crypto[id] == nil {
		m.crypto[id] = make(map[string]struct{})
	}
	m.crypto[id][sym] = struct{}{}
}

func (m *memStore) unsubscribeCrypto(id int64, sym string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.crypto[id], sym)
}

func (m *memStore) getCrypto(id int64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for k := range m.crypto[id] {
		out = append(out, k)
	}
	return out
}

// ─── test service builders ────────────────────────────────────────────────────

// startHistoryService returns an httptest.Server that simulates the history-service.
func startHistoryService(t *testing.T) *httptest.Server {
	t.Helper()

	cbrRates := []map[string]any{
		{"currency_code": "USD", "currency_name": "US Dollar", "nominal": 1, "value": 90.5, "previous": 89.0},
		{"currency_code": "EUR", "currency_name": "Euro", "nominal": 1, "value": 98.2, "previous": 97.5},
	}
	cryptoRates := []map[string]any{
		{"symbol": "BTCUSDT", "close": 41000.0, "price_rub": 3690000.0},
		{"symbol": "BTCUSDT", "close": 42000.0, "price_rub": 3780000.0},
	}
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}

	mux := http.NewServeMux()

	mux.HandleFunc("/history/cbr", func(w http.ResponseWriter, r *http.Request) {
		if date := r.URL.Query().Get("date"); date != "" {
			_, err := time.Parse("2006-01-02", date)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date"})
				return
			}
		}
		writeJSON(w, http.StatusOK, cbrRates)
	})

	mux.HandleFunc("/history/crypto", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("symbol") == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol required"})
			return
		}
		writeJSON(w, http.StatusOK, cryptoRates)
	})

	mux.HandleFunc("/history/crypto/symbols", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, symbols)
	})

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// startNotificationService returns an httptest.Server that simulates the notification-service.
func startNotificationService(t *testing.T, store *memStore) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/subscriptions/cbr", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				TelegramID int64  `json:"telegram_id"`
				Value      string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
				return
			}
			store.subscribeCBR(body.TelegramID, body.Value)
			w.WriteHeader(http.StatusNoContent)

		case http.MethodDelete:
			var body struct {
				TelegramID int64  `json:"telegram_id"`
				Value      string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
				return
			}
			store.unsubscribeCBR(body.TelegramID, body.Value)
			w.WriteHeader(http.StatusNoContent)

		case http.MethodGet:
			tidStr := r.URL.Query().Get("telegram_id")
			tid, err := strconv.ParseInt(tidStr, 10, 64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid telegram_id"})
				return
			}
			writeJSON(w, http.StatusOK, store.getCBR(tid))
		}
	})

	mux.HandleFunc("/subscriptions/crypto", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				TelegramID int64  `json:"telegram_id"`
				Value      string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
				return
			}
			store.subscribeCrypto(body.TelegramID, body.Value)
			w.WriteHeader(http.StatusNoContent)

		case http.MethodDelete:
			var body struct {
				TelegramID int64  `json:"telegram_id"`
				Value      string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
				return
			}
			store.unsubscribeCrypto(body.TelegramID, body.Value)
			w.WriteHeader(http.StatusNoContent)

		case http.MethodGet:
			tidStr := r.URL.Query().Get("telegram_id")
			tid, err := strconv.ParseInt(tidStr, 10, 64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid telegram_id"})
				return
			}
			writeJSON(w, http.StatusOK, store.getCrypto(tid))
		}
	})

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// startAPIGateway returns an httptest.Server that acts as a minimal gateway.
// It routes /history/* and /rates/* to historySrv, /notifications/* to notifSrv.
func startAPIGateway(t *testing.T, historySrv, notifSrv *httptest.Server) *httptest.Server {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}

	forward := func(w http.ResponseWriter, r *http.Request, targetURL string) {
		req, err := http.NewRequest(r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("pong"))
	})

	// history routes
	mux.HandleFunc("/history/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.RequestURI(), "/history")
		forward(w, r, historySrv.URL+"/history"+path)
	})

	// rates shortcuts
	mux.HandleFunc("/rates/cbr", func(w http.ResponseWriter, r *http.Request) {
		forward(w, r, historySrv.URL+"/history/cbr?"+r.URL.RawQuery)
	})
	mux.HandleFunc("/rates/crypto/symbols", func(w http.ResponseWriter, r *http.Request) {
		forward(w, r, historySrv.URL+"/history/crypto/symbols")
	})
	mux.HandleFunc("/rates/crypto/history", func(w http.ResponseWriter, r *http.Request) {
		forward(w, r, historySrv.URL+"/history/crypto?"+r.URL.RawQuery)
	})

	// notification routes
	mux.HandleFunc("/notifications/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.RequestURI(), "/notifications")
		forward(w, r, notifSrv.URL+path)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// writeJSON is a helper to write JSON responses in test servers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ─── E2E tests ────────────────────────────────────────────────────────────────

// TestE2E_HealthChecks verifies that /ping returns 200 "pong" through the gateway.
func TestE2E_HealthChecks(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	resp, err := http.Get(gw.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "pong" {
		t.Errorf("expected 'pong', got %q", string(body))
	}
}

// TestE2E_CBRHistory_fullFlow tests the complete path:
// HTTP client → API Gateway → history-service stub → JSON rates returned.
func TestE2E_CBRHistory_fullFlow(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	resp, err := http.Get(gw.URL + "/history/cbr?date=2024-01-15")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rates []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatal(err)
	}
	if len(rates) != 2 {
		t.Errorf("expected 2 rates, got %d", len(rates))
	}
}

// TestE2E_CryptoHistory_fullFlow tests the crypto rates endpoint.
func TestE2E_CryptoHistory_fullFlow(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	resp, err := http.Get(gw.URL + "/rates/crypto/history?symbol=BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rates []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatal(err)
	}
	if len(rates) == 0 {
		t.Error("expected at least one rate")
	}
}

// TestE2E_CryptoSymbols_fullFlow tests the symbols list endpoint.
func TestE2E_CryptoSymbols_fullFlow(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	resp, err := http.Get(gw.URL + "/rates/crypto/symbols")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var symbols []string
	if err := json.NewDecoder(resp.Body).Decode(&symbols); err != nil {
		t.Fatal(err)
	}
	if len(symbols) == 0 {
		t.Error("expected at least one symbol")
	}
}

// TestE2E_SubscriptionWorkflow_CBR tests the full CRUD lifecycle for CBR subscriptions:
// subscribe → list (contains) → unsubscribe → list (empty).
func TestE2E_SubscriptionWorkflow_CBR(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	const userID = int64(100)

	// 1. Subscribe to USD
	body, _ := json.Marshal(map[string]any{"telegram_id": userID, "value": "USD"})
	resp, err := http.Post(gw.URL+"/notifications/subscriptions/cbr",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("subscribe: expected 204, got %d", resp.StatusCode)
	}

	// 2. List — should contain USD
	resp, err = http.Get(fmt.Sprintf("%s/notifications/subscriptions/cbr?telegram_id=%d", gw.URL, userID))
	if err != nil {
		t.Fatal(err)
	}
	var subs []string
	json.NewDecoder(resp.Body).Decode(&subs)
	resp.Body.Close()
	if len(subs) != 1 || subs[0] != "USD" {
		t.Errorf("expected [USD], got %v", subs)
	}

	// 3. Unsubscribe
	delBody, _ := json.Marshal(map[string]any{"telegram_id": userID, "value": "USD"})
	req, _ := http.NewRequest(http.MethodDelete,
		gw.URL+"/notifications/subscriptions/cbr",
		bytes.NewReader(delBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unsubscribe: expected 204, got %d", resp.StatusCode)
	}

	// 4. List — should be empty
	resp, err = http.Get(fmt.Sprintf("%s/notifications/subscriptions/cbr?telegram_id=%d", gw.URL, userID))
	if err != nil {
		t.Fatal(err)
	}
	var subsAfter []string
	json.NewDecoder(resp.Body).Decode(&subsAfter)
	resp.Body.Close()
	if len(subsAfter) != 0 {
		t.Errorf("expected empty list after unsubscribe, got %v", subsAfter)
	}
}

// TestE2E_SubscriptionWorkflow_Crypto tests the full CRUD lifecycle for crypto subscriptions.
func TestE2E_SubscriptionWorkflow_Crypto(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	const userID = int64(200)

	// 1. Subscribe to BTCUSDT
	body, _ := json.Marshal(map[string]any{"telegram_id": userID, "value": "BTCUSDT"})
	resp, err := http.Post(gw.URL+"/notifications/subscriptions/crypto",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("subscribe crypto: expected 204, got %d", resp.StatusCode)
	}

	// 2. List — should contain BTCUSDT
	resp, err = http.Get(fmt.Sprintf("%s/notifications/subscriptions/crypto?telegram_id=%d", gw.URL, userID))
	if err != nil {
		t.Fatal(err)
	}
	var subs []string
	json.NewDecoder(resp.Body).Decode(&subs)
	resp.Body.Close()

	found := false
	for _, s := range subs {
		if s == "BTCUSDT" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected BTCUSDT in %v", subs)
	}

	// 3. Unsubscribe
	delBody, _ := json.Marshal(map[string]any{"telegram_id": userID, "value": "BTCUSDT"})
	req, _ := http.NewRequest(http.MethodDelete,
		gw.URL+"/notifications/subscriptions/crypto",
		bytes.NewReader(delBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unsubscribe crypto: expected 204, got %d", resp.StatusCode)
	}
}

// TestE2E_InvalidDate_propagatesError verifies that a bad date parameter
// results in a 400 from the history-service, passed through the gateway.
func TestE2E_InvalidDate_propagatesError(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	resp, err := http.Get(gw.URL + "/history/cbr?date=not-a-date")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestE2E_ConcurrentRequests verifies that the gateway handles concurrent
// requests correctly with no data races.
func TestE2E_ConcurrentRequests(t *testing.T) {
	store := newMemStore()
	historySrv := startHistoryService(t)
	notifSrv := startNotificationService(t, store)
	gw := startAPIGateway(t, historySrv, notifSrv)

	const concurrency = 20
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(gw.URL + "/ping")
			if err != nil {
				errs <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("expected 200, got %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}
