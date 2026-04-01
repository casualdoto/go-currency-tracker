package main

import (
	"log"
	"net/http"
	"os"

	"github.com/casualdoto/go-currency-tracker/microservices/auth-service/internal/config"
	"github.com/casualdoto/go-currency-tracker/microservices/auth-service/internal/handler"
	"github.com/casualdoto/go-currency-tracker/microservices/auth-service/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()

	db, err := storage.NewPostgresDB(storage.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		log.Fatalf("failed to init schema: %v", err)
	}

	h := handler.New(db, cfg.JWTSecret)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Get("/validate", h.Validate)

	addr := ":" + cfg.ServerPort
	log.Printf("Auth Service listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
