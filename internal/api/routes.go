package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes() http.Handler {
	r := chi.NewRouter()

	r.Get("/ping", PingHandler)
	r.Get("/info", InfoHandler)

	// Тут будут другие маршруты (например, /rates, /metrics...)

	return r
}
