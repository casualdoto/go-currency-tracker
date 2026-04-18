package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/casualdoto/go-currency-tracker/microservices/api-gateway/internal/config"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func newTestGateway(historySrvURL, notifSrvURL string) *Gateway {
	return &Gateway{
		cfg: &config.Config{
			HistoryServiceURL:      historySrvURL,
			NotificationServiceURL: notifSrvURL,
		},
		httpClient: &http.Client{},
	}
}

func doRequest(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// ─── corsMiddleware ───────────────────────────────────────────────────────────

func TestCORSMiddleware_setsHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	corsMiddleware(next).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", got, "*")
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "GET") {
		t.Errorf("Access-Control-Allow-Methods should contain GET, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Errorf("Access-Control-Allow-Headers should contain Content-Type, got %q", got)
	}
}

func TestCORSMiddleware_preflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rr := httptest.NewRecorder()
	corsMiddleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("preflight: expected 204, got %d", rr.Code)
	}
	if called {
		t.Error("next handler must NOT be called for OPTIONS preflight")
	}
}

func TestCORSMiddleware_nonOptions_callsNext(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	corsMiddleware(next).ServeHTTP(rr, req)

	if !called {
		t.Error("next handler must be called for non-OPTIONS requests")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("expected status from next handler (418), got %d", rr.Code)
	}
}

// ─── /ping ────────────────────────────────────────────────────────────────────

func TestPing(t *testing.T) {
	gw := newTestGateway("http://localhost:8084", "http://localhost:8085")
	rr := doRequest(t, gw.Routes(), http.MethodGet, "/ping")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "pong" {
		t.Errorf("expected body %q, got %q", "pong", body)
	}
}

// ─── proxyTo ──────────────────────────────────────────────────────────────────

func TestProxyTo_forwardsResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	gw := newTestGateway(upstream.URL, upstream.URL)
	handler := gw.proxyTo(upstream.URL + "/some-path")

	req := httptest.NewRequest(http.MethodGet, "/rates/cbr", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}

func TestProxyTo_preservesQueryParams(t *testing.T) {
	var receivedQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	gw := newTestGateway(upstream.URL, upstream.URL)
	handler := gw.proxyTo(upstream.URL + "/path")

	req := httptest.NewRequest(http.MethodGet, "/rates/cbr?date=2024-01-15&currency=USD", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if !strings.Contains(receivedQuery, "date=2024-01-15") {
		t.Errorf("query params not forwarded: %q", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "currency=USD") {
		t.Errorf("query params not forwarded: %q", receivedQuery)
	}
}

func TestProxyTo_upstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close() // закрываем до запроса

	gw := newTestGateway(upstreamURL, upstreamURL)
	handler := gw.proxyTo(upstreamURL + "/path")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rr.Code)
	}
}

func TestProxyTo_upstreamNon200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	gw := newTestGateway(upstream.URL, upstream.URL)
	handler := gw.proxyTo(upstream.URL + "/path")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 to be proxied through, got %d", rr.Code)
	}
}

// ─── reverseProxy ─────────────────────────────────────────────────────────────

func TestReverseProxy_stripsPrefix(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	gw := newTestGateway(upstream.URL, upstream.URL)
	handler := gw.reverseProxy(upstream.URL, "/history")

	req := httptest.NewRequest(http.MethodGet, "/history/cbr", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if receivedPath != "/cbr" {
		t.Errorf("expected upstream to receive /cbr, got %q", receivedPath)
	}
}

func TestReverseProxy_rootPath(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	gw := newTestGateway(upstream.URL, upstream.URL)
	handler := gw.reverseProxy(upstream.URL, "/history")

	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if receivedPath != "/" {
		t.Errorf("expected upstream to receive /, got %q", receivedPath)
	}
}

func TestReverseProxy_upstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close()

	gw := newTestGateway(upstreamURL, upstreamURL)
	handler := gw.reverseProxy(upstreamURL, "/history")

	req := httptest.NewRequest(http.MethodGet, "/history/cbr", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rr.Code)
	}
}

// ─── Routes integration ───────────────────────────────────────────────────────

func TestRoutes_proxiesHistoryPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "history-ok")
	}))
	defer upstream.Close()

	gw := newTestGateway(upstream.URL, upstream.URL)
	rr := doRequest(t, gw.Routes(), http.MethodGet, "/rates/cbr")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
