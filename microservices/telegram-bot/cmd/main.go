package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/microservices/telegram-bot/internal/bot"
	"github.com/casualdoto/go-currency-tracker/microservices/telegram-bot/internal/config"
)

func main() {
	cfg := config.Load()

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}

	log.Println("Telegram Bot Service: starting")
	b.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Telegram Bot Service: shutting down")
	b.Stop()
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
