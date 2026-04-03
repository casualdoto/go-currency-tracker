package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/cbrbackfill"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/config"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/cryptobackfill"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/handler"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/subscriber"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()

	// Connect to PostgreSQL (CBR rates)
	pg, err := storage.NewPostgresDB(storage.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pg.Close()

	if err := pg.InitSchema(); err != nil {
		log.Fatalf("failed to init postgres schema: %v", err)
	}

	// Connect to ClickHouse (crypto rates)
	ch, err := storage.NewClickHouseDB(storage.ClickHouseConfig{
		Host:     cfg.CHHost,
		Port:     cfg.CHPort,
		Database: cfg.CHDatabase,
		User:     cfg.CHUser,
		Password: cfg.CHPassword,
	})
	if err != nil {
		log.Fatalf("failed to connect to clickhouse: %v", err)
	}
	defer ch.Close()

	if err := ch.InitSchema(); err != nil {
		log.Fatalf("failed to init clickhouse schema: %v", err)
	}

	// Start Kafka subscriber in background
	sub := subscriber.New(cfg.KafkaBrokers, pg, ch)
	go func() {
		log.Println("History Service: starting Kafka subscriber")
		if err := sub.Run(); err != nil {
			log.Printf("subscriber error: %v", err)
		}
	}()

	// Setup HTTP router (optional CBR archive client when CBR_BASE_URL is set)
	cbrClient := cbrbackfill.New(cfg.CBRBaseURL)
	cryptoBackfill := cryptobackfill.New(cfg.BinanceAPIBase, cbrClient)
	h := handler.New(pg, ch, cbrClient, cryptoBackfill)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CBR history endpoints
	r.Get("/history/cbr", h.GetCBRHistory)
	r.Get("/history/cbr/range", h.GetCBRHistoryRange)

	// Crypto history endpoints (backed by ClickHouse)
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
