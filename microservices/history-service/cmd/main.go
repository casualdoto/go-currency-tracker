package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/config"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/handler"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/subscriber"
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

	// Start Kafka subscriber in background
	sub := subscriber.New(cfg.KafkaBrokers, db)
	go func() {
		log.Println("History Service: starting Kafka subscriber")
		if err := sub.Run(); err != nil {
			log.Printf("subscriber error: %v", err)
		}
	}()

	// Setup HTTP router
	h := handler.New(db)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CBR history endpoints
	r.Get("/history/cbr", h.GetCBRHistory)
	r.Get("/history/cbr/range", h.GetCBRHistoryRange)

	// Crypto history endpoints
	r.Get("/history/crypto", h.GetCryptoHistory)
	r.Get("/history/crypto/range", h.GetCryptoHistoryRange)
	r.Get("/history/crypto/symbols", h.GetCryptoSymbols)

	// Health
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	addr := ":" + cfg.ServerPort
	log.Printf("History Service listening on %s", addr)

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("History Service: shutting down")
}
