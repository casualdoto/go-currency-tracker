package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/casualdoto/go-currency-tracker/internal/storage"
)

// Function to get the project root path
func getProjectRoot() string {
	// Try to determine the project directory
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	// Get the directory of the current file
	dir := filepath.Dir(filename)

	// Go up two levels (internal/api -> internal -> root)
	return filepath.Dir(filepath.Dir(dir))
}

// SetupRoutes configures and returns an HTTP request router.
// Registers all API handlers and middleware.
func SetupRoutes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(CORSMiddleware)

	// Basic routes
	r.Get("/ping", PingHandler)
	r.Get("/info", InfoHandler)

	// Routes for currency rates
	r.Get("/rates/cbr", CBRRatesHandler)             // All rates (with optional date parameter)
	r.Get("/rates/cbr/currency", CBRCurrencyHandler) // Specific currency rate

	// Static OpenAPI documentation
	r.Get("/api/docs", SwaggerUIHandler)
	r.Get("/api/openapi", OpenAPIHandler)

	// Add handler for root documentation path
	r.Get("/api", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/docs", http.StatusFound)
	})

	return r
}

// SetupRoutesWithDB configures API routes with database access
func SetupRoutesWithDB(db *storage.PostgresDB) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(CORSMiddleware)

	// Add database to context
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "db", db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// Basic endpoints
	r.Get("/ping", PingHandler)
	r.Get("/info", InfoHandler)

	// CBR rates endpoints
	r.Get("/rates/cbr", CBRRatesHandler)
	r.Get("/rates/cbr/currency", CBRCurrencyHandler)

	// API documentation
	r.Get("/api/docs", SwaggerUIHandler)
	r.Get("/api/openapi", OpenAPIHandler)
	r.Get("/api", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/docs", http.StatusFound)
	})

	return r
}

// corsMiddleware is defined in handlers.go

// SwaggerUIHandler serves the Swagger UI HTML page
func SwaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	// Serve HTML page with Swagger UI for documentation
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Go Currency Tracker API - Documentation</title>
  <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.0.0/swagger-ui.css">
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
    .swagger-ui .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>

  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.0.0/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "/api/openapi",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// OpenAPIHandler serves the OpenAPI specification
func OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	var docsPath string

	// Try to find documentation file in several locations
	// 1. First check relative path from current working directory
	workDir, err := os.Getwd()
	if err == nil {
		path := filepath.Join(workDir, "openapi", "openapi.json")
		if _, err := os.Stat(path); err == nil {
			docsPath = path
		}
	}

	// 2. If not found, try to find relative to project root
	if docsPath == "" {
		projectRoot := getProjectRoot()
		if projectRoot != "" {
			path := filepath.Join(projectRoot, "openapi", "openapi.json")
			if _, err := os.Stat(path); err == nil {
				docsPath = path
			}
		}
	}

	// 3. If still not found, try to find relative to executable file
	if docsPath == "" {
		execPath, err := os.Executable()
		if err == nil {
			execDir := filepath.Dir(execPath)
			path := filepath.Join(execDir, "openapi", "openapi.json")
			if _, err := os.Stat(path); err == nil {
				docsPath = path
			}
		}
	}

	// If file not found in any location
	if docsPath == "" {
		http.Error(w, "API documentation not found", http.StatusNotFound)
		return
	}

	// Set Content-Type header for JSON
	w.Header().Set("Content-Type", "application/json")

	// Send file
	http.ServeFile(w, r, docsPath)
}
