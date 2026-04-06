package gateway

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/api-gateway/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Gateway holds service URLs and the HTTP client.
type Gateway struct {
	cfg        *config.Config
	httpClient *http.Client
}

func New(cfg *config.Config) *Gateway {
	// History-service crypto range can chain two Binance calls plus ClickHouse; 30s caused frequent gateway timeouts.
	return &Gateway{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Routes builds and returns the chi router.
func (g *Gateway) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// Health check
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	// History routes — public (read-only data)
	r.Mount("/history", g.reverseProxy(g.cfg.HistoryServiceURL, "/history"))

	// Current rates via History Service
	r.Get("/rates/cbr", g.proxyTo(g.cfg.HistoryServiceURL+"/history/cbr"))
	r.Get("/rates/cbr/range", g.proxyTo(g.cfg.HistoryServiceURL+"/history/cbr/range"))
	r.Get("/rates/crypto/symbols", g.proxyTo(g.cfg.HistoryServiceURL+"/history/crypto/symbols"))
	r.Get("/rates/crypto/history", g.proxyTo(g.cfg.HistoryServiceURL+"/history/crypto"))
	r.Get("/rates/crypto/history/range", g.proxyTo(g.cfg.HistoryServiceURL+"/history/crypto/range"))

	// Notification / subscription routes
	r.Mount("/notifications", g.reverseProxy(g.cfg.NotificationServiceURL, "/notifications"))

	return r
}

// reverseProxy creates a reverse proxy that strips the given prefix.
func (g *Gateway) reverseProxy(targetBase, stripPrefix string) http.Handler {
	target, err := url.Parse(targetBase)
	if err != nil {
		log.Fatalf("invalid upstream URL %q: %v", targetBase, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error for %s: %v", r.URL, err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, stripPrefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		r.Header.Set("X-Forwarded-Host", r.Host)
		proxy.ServeHTTP(w, r)
	})
}

// proxyTo forwards the request to the given full URL, preserving query params.
func (g *Gateway) proxyTo(targetURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest(r.Method, targetURL+"?"+r.URL.RawQuery, r.Body)
		if err != nil {
			http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()

		resp, err := g.httpClient.Do(req)
		if err != nil {
			log.Printf("proxyTo %s error: %v", targetURL, err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
