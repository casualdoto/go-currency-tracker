package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Функция для получения пути к корню проекта
func getProjectRoot() string {
	// Пытаемся определить директорию проекта
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	// Получаем директорию текущего файла
	dir := filepath.Dir(filename)

	// Поднимаемся на два уровня вверх (internal/api -> internal -> root)
	return filepath.Dir(filepath.Dir(dir))
}

func SetupRoutes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(CORSMiddleware)

	// Базовые маршруты
	r.Get("/ping", PingHandler)
	r.Get("/info", InfoHandler)

	// Маршруты для работы с курсами валют
	r.Get("/rates/cbr", CBRRatesHandler)             // Все курсы (с опциональным параметром даты)
	r.Get("/rates/cbr/currency", CBRCurrencyHandler) // Курс конкретной валюты

	// Статическая документация OpenAPI
	r.Get("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		// Отдаем HTML страницу с Swagger UI для просмотра документации
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
	})

	// Эндпоинт для получения OpenAPI спецификации
	r.Get("/api/openapi", func(w http.ResponseWriter, r *http.Request) {
		var docsPath string

		// Пытаемся найти файл документации в нескольких местах
		// 1. Сначала проверяем относительный путь от текущего рабочего каталога
		workDir, err := os.Getwd()
		if err == nil {
			path := filepath.Join(workDir, "api", "openapi.json")
			if _, err := os.Stat(path); err == nil {
				docsPath = path
			}
		}

		// 2. Если не нашли, пробуем найти относительно корня проекта
		if docsPath == "" {
			projectRoot := getProjectRoot()
			if projectRoot != "" {
				path := filepath.Join(projectRoot, "api", "openapi.json")
				if _, err := os.Stat(path); err == nil {
					docsPath = path
				}
			}
		}

		// 3. Если все равно не нашли, пробуем найти относительно исполняемого файла
		if docsPath == "" {
			execPath, err := os.Executable()
			if err == nil {
				execDir := filepath.Dir(execPath)
				path := filepath.Join(execDir, "api", "openapi.json")
				if _, err := os.Stat(path); err == nil {
					docsPath = path
				}
			}
		}

		// Если файл не найден ни в одном из мест
		if docsPath == "" {
			http.Error(w, "Документация API не найдена", http.StatusNotFound)
			return
		}

		// Устанавливаем заголовок Content-Type для JSON
		w.Header().Set("Content-Type", "application/json")

		// Отправляем файл
		http.ServeFile(w, r, docsPath)
	})

	// Добавляем обработчик для корневого пути документации
	r.Get("/api", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/docs", http.StatusFound)
	})

	return r
}
