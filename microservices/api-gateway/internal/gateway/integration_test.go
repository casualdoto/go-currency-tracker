//go:build integration

package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/api-gateway/internal/config"
)

// newIntegrationGateway starts a real gateway httptest.Server that proxies to
// the given history and notification upstream URLs.
func newIntegrationGateway(t *testing.T, historyURL, notifURL string) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		HistoryServiceURL:      historyURL,
		NotificationServiceURL: notifURL,
		ServerPort:             "8080",
	}
	gw := &Gateway{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	srv := httptest.NewServer(gw.Routes())
	t.Cleanup(srv.Close)
	return srv
}

// echoUpstream returns a server that writes its received path and query as JSON.
func echoUpstream(t *testing.T) (*httptest.Server, *string, *string) {
	t.Helper()
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": path, "query": query})
	}))
	t.Cleanup(srv.Close)
	return srv, &path, &query
}

// TestIntegration_Ping verifies the gateway health check endpoint.
func TestIntegration_Ping(t *testing.T) {
	gw := newIntegrationGateway(t, "http://127.0.0.1:1", "http://127.0.0.1:1")

	resp, err := http.Get(gw.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestIntegration_HistoryProxy_forwardsToUpstream verifies path prefix stripping.
func TestIntegration_HistoryProxy_forwardsToUpstream(t *testing.T) {
	upstream, receivedPath, _ := echoUpstream(t)
	gw := newIntegrationGateway(t, upstream.URL, "http://127.0.0.1:1")

	resp, err := http.Get(gw.URL + "/history/cbr")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if *receivedPath != "/cbr" {
		t.Errorf("expected upstream path /cbr, got %q", *receivedPath)
	}
}

// TestIntegration_HistoryProxy_responsePassthrough verifies that the gateway
// passes upstream response body and status code through unchanged.
func TestIntegration_HistoryProxy_responsePassthrough(t *testing.T) {
	payload := `[{"currency_code":"USD","value":90.5}]`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, payload)
	}))
	defer upstream.Close()

	gw := newIntegrationGateway(t, upstream.URL, "http://127.0.0.1:1")
	resp, err := http.Get(gw.URL + "/history/cbr")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 item, got %d", len(result))
	}
}

// TestIntegration_NotificationsProxy_forwardsToUpstream verifies that
// notification routes are correctly proxied with prefix stripped.
func TestIntegration_NotificationsProxy_forwardsToUpstream(t *testing.T) {
	upstream, receivedPath, _ := echoUpstream(t)
	gw := newIntegrationGateway(t, "http://127.0.0.1:1", upstream.URL)

	resp, err := http.Get(gw.URL + "/notifications/subscriptions/cbr?telegram_id=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if *receivedPath != "/subscriptions/cbr" {
		t.Errorf("expected /subscriptions/cbr, got %q", *receivedPath)
	}
}

// TestIntegration_CORS_headersPresent verifies CORS headers on all responses.
func TestIntegration_CORS_headersPresent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	gw := newIntegrationGateway(t, upstream.URL, upstream.URL)

	endpoints := []string{"/ping", "/history/cbr", "/notifications/subscriptions/cbr"}
	for _, ep := range endpoints {
		resp, err := http.Get(gw.URL + ep)
		if err != nil {
			t.Errorf("%s: %v", ep, err)
			continue
		}
		resp.Body.Close()
		if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "*" {
			t.Errorf("%s: expected Access-Control-Allow-Origin: *, got %q", ep, v)
		}
	}
}

// TestIntegration_CORS_preflightHandled verifies OPTIONS preflight returns 204
// without calling the upstream.
func TestIntegration_CORS_preflightHandled(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer upstream.Close()

	gw := newIntegrationGateway(t, upstream.URL, upstream.URL)

	req, _ := http.NewRequest(http.MethodOptions, gw.URL+"/history/cbr", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	if called {
		t.Error("upstream must not be called during preflight")
	}
}

// TestIntegration_RatesCBR_queryParamsForwarded verifies that query parameters
// are forwarded from the client through the gateway to the upstream.
func TestIntegration_RatesCBR_queryParamsForwarded(t *testing.T) {
	upstream, _, receivedQuery := echoUpstream(t)
	gw := newIntegrationGateway(t, upstream.URL, "http://127.0.0.1:1")

	resp, err := http.Get(gw.URL + "/rates/cbr?date=2024-01-15")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if !strings.Contains(*receivedQuery, "date=2024-01-15") {
		t.Errorf("expected date=2024-01-15 in upstream query, got %q", *receivedQuery)
	}
}

// TestIntegration_UpstreamDown_returns502 verifies that a closed upstream
// causes the gateway to return 502 Bad Gateway.
func TestIntegration_UpstreamDown_returns502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close()

	gw := newIntegrationGateway(t, upstreamURL, "http://127.0.0.1:1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(gw.URL + "/rates/cbr")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", resp.StatusCode)
	}
}
