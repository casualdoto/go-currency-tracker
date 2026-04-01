package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/microservices/notification-service/internal/config"
	"github.com/casualdoto/go-currency-tracker/microservices/notification-service/internal/handler"
	"github.com/casualdoto/go-currency-tracker/microservices/notification-service/internal/store"
	"github.com/casualdoto/go-currency-tracker/microservices/notification-service/internal/subscriber"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()

	redisStore := store.NewRedis(cfg.RedisAddr)

	sub := subscriber.New(cfg.KafkaBrokers, redisStore, cfg.TelegramBotToken)
	go func() {
		log.Println("Notification Service: starting Kafka subscriber")
		if err := sub.Run(); err != nil {
			log.Printf("subscriber error: %v", err)
		}
	}()

	h := handler.New(redisStore)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Subscription management
	r.Post("/subscriptions/cbr", h.SubscribeCBR)
	r.Delete("/subscriptions/cbr", h.UnsubscribeCBR)
	r.Get("/subscriptions/cbr", h.ListCBRSubscriptions)
	r.Post("/subscriptions/crypto", h.SubscribeCrypto)
	r.Delete("/subscriptions/crypto", h.UnsubscribeCrypto)
	r.Get("/subscriptions/crypto", h.ListCryptoSubscriptions)

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("pong")) })

	addr := ":" + cfg.ServerPort
	log.Printf("Notification Service listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Notification Service: shutting down")
}
